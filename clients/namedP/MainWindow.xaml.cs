using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Text.Json;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;
using Microsoft.Win32;

namespace NamedP;

public partial class MainWindow : Window
{
    Process? engine, vpn;
    CancellationTokenSource? session;
    bool fullVpn = true, connecting, connected, stopping, closing;
    DateTime connectedAt;
    readonly DispatcherTimer ticker = new() { Interval = TimeSpan.FromSeconds(1) };
    readonly string settings = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "namedP", "theme.txt");
    public MainWindow()
    {
        InitializeComponent();
        var version = System.Reflection.CustomAttributeExtensions.GetCustomAttribute<System.Reflection.AssemblyInformationalVersionAttribute>(typeof(MainWindow).Assembly)?.InformationalVersion.Split('+')[0] ?? "development";
        ClientVersion.Text = "namedP · " + version;
        try { if (File.Exists(settings) && int.TryParse(File.ReadAllText(settings), out int index)) Theme.SelectedIndex = Math.Clamp(index, 0, 2); } catch (IOException) { }
        ApplyTheme(); UpdateModes();
        SystemEvents.UserPreferenceChanged += SystemThemeChanged;
        ticker.Tick += (_, _) => { if (connected) Elapsed.Text = (DateTime.UtcNow - connectedAt).ToString(@"hh\:mm\:ss"); };
        ticker.Start(); Closing += OnClosing;
    }
    void SystemThemeChanged(object sender, UserPreferenceChangedEventArgs e) => Dispatcher.BeginInvoke(new Action(ApplyTheme));
    void ThemeChanged(object sender, SelectionChangedEventArgs e)
    {
        if (!IsInitialized) return;
        ApplyTheme();
        try { Directory.CreateDirectory(Path.GetDirectoryName(settings)!); File.WriteAllText(settings, Theme.SelectedIndex.ToString()); } catch (IOException) { }
    }
    void ApplyTheme()
    {
        bool dark = Theme.SelectedIndex == 1;
        if (Theme.SelectedIndex == 0)
        {
            using var key = Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize");
            dark = key?.GetValue("AppsUseLightTheme") is int value && value == 0;
        }
        var colors = dark
            ? new[] { "#10161D", "#19222C", "#EDF3F4", "#97A6B5", "#2B3946", "#62DDC0" }
            : new[] { "#F5F5F0", "#FFFFFF", "#1A302C", "#667A75", "#DBE4DD", "#4BC4A5" };
        string[] names = ["Bg", "Surface", "Text", "Muted", "Line", "Accent"];
        for (int i = 0; i < names.Length; i++) Application.Current.Resources[names[i]] = new SolidColorBrush((Color)ColorConverter.ConvertFromString(colors[i]));
        if (VpnMode != null) UpdateModes();
    }
    void ChooseVpn(object sender, RoutedEventArgs e) { fullVpn = true; UpdateModes(); }
    void ChooseProxy(object sender, RoutedEventArgs e) { fullVpn = false; UpdateModes(); }
    void UpdateModes()
    {
        VpnMode.BorderBrush = (Brush)FindResource(fullVpn ? "Accent" : "Line");
        ProxyMode.BorderBrush = (Brush)FindResource(fullVpn ? "Line" : "Accent");
        Hint.Text = fullVpn ? "Route device TCP + UDP through your namedICMP server." : "Only apps using the local SOCKS5 proxy are tunneled.";
    }
    void PasteLink(object sender, RoutedEventArgs e)
    {
        try { if (Clipboard.ContainsText()) Link.Password = Clipboard.GetText().Trim(); } catch (System.Runtime.InteropServices.COMException) { Error.Text = "Clipboard busy. Try again."; }
    }
    void OpenGithub(object sender, RoutedEventArgs e) => Process.Start(new ProcessStartInfo("https://github.com/namedkenzo/NamedICMP") { UseShellExecute = true });
    static Process Start(string file, params string[] args)
    {
        var info = new ProcessStartInfo(Path.Combine(AppContext.BaseDirectory, file)) { UseShellExecute = false, CreateNoWindow = true, RedirectStandardInput = true, RedirectStandardOutput = true, RedirectStandardError = true, WorkingDirectory = AppContext.BaseDirectory };
        foreach (string arg in args) info.ArgumentList.Add(arg);
        return Process.Start(info) ?? throw new IOException("Could not start " + file);
    }
    async Task Watch(Process process, TaskCompletionSource<JsonElement> ready, CancellationToken ct)
    {
        // Errors intentionally exclude the profile and encryption key.
        var errors = process.StandardError.ReadToEndAsync(ct);
        try
        {
            while (await process.StandardOutput.ReadLineAsync(ct) is string line)
            {
                using var doc = JsonDocument.Parse(line);
                var root = doc.RootElement;
                string? kind = root.GetProperty("event").GetString();
                if (kind == "ready") ready.TrySetResult(root.Clone());
                if (kind == "stats") await Dispatcher.InvokeAsync(() => Traffic.Text = $"↑ {Bytes(root.GetProperty("upload").GetUInt64())}     ↓ {Bytes(root.GetProperty("download").GetUInt64())}");
            }
            await process.WaitForExitAsync(ct);
            if (!ct.IsCancellationRequested)
            {
                string detail = await errors;
                if (detail.Length > 350) detail = detail[..350];
                var error = new IOException(string.IsNullOrWhiteSpace(detail) ? "Connection process stopped." : detail.Trim());
                ready.TrySetException(error);
                if (connected) await Dispatcher.InvokeAsync(async () => { Error.Text = error.Message; await Disconnect(); });
            }
        }
        catch (OperationCanceledException) { ready.TrySetCanceled(); }
        catch (Exception ex) { ready.TrySetException(ex); if (connected) await Dispatcher.InvokeAsync(async () => { Error.Text = "Connection monitor stopped."; await Disconnect(); }); }
    }
    async void ToggleConnection(object sender, RoutedEventArgs e)
    {
        if (connected || connecting) { await Disconnect(); return; }
        if (string.IsNullOrWhiteSpace(Link.Password)) { Error.Text = "Paste the complete pingtunnel:// connection link."; return; }
        Error.Text = ""; connecting = true; session = new(); var ct = session.Token;
        Dog.State = "connecting"; StateLabel.Text = "●  Checking your tunnel…"; ConnectButton.Content = "Cancel";
        VpnMode.IsEnabled = ProxyMode.IsEnabled = Link.IsEnabled = false;
        try
        {
            engine = Start("namedp-engine.exe");
            var ready = new TaskCompletionSource<JsonElement>(TaskCreationOptions.RunContinuationsAsynchronously);
            _ = Watch(engine, ready, ct);
            await engine.StandardInput.WriteLineAsync(JsonSerializer.Serialize(new { link = Link.Password }));
            await engine.StandardInput.FlushAsync();
            JsonElement info = await ready.Task.WaitAsync(TimeSpan.FromSeconds(25), ct);
            string ip = info.GetProperty("server_ip").GetString()!;
            if (!IPAddress.TryParse(ip, out var address) || address.AddressFamily != System.Net.Sockets.AddressFamily.InterNetwork) throw new IOException("Invalid tunnel server address.");
            int port = info.GetProperty("port").GetInt32();
            if (fullVpn)
            {
                StateLabel.Text = "●  Preparing your private route…";
                string ps = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), @"WindowsPowerShell\v1.0\powershell.exe");
                vpn = Start(ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", Path.Combine(AppContext.BaseDirectory, "vpn.ps1"), "-ParentPid", Environment.ProcessId.ToString(), "-EnginePid", engine.Id.ToString(), "-ServerIP", ip, "-Port", port.ToString());
                var vpnReady = new TaskCompletionSource<JsonElement>(TaskCreationOptions.RunContinuationsAsynchronously);
                _ = Watch(vpn, vpnReady, ct);
                await vpnReady.Task.WaitAsync(TimeSpan.FromSeconds(30), ct);
            }
            ct.ThrowIfCancellationRequested();
            connecting = false; connected = true; connectedAt = DateTime.UtcNow;
            Dog.State = "connected"; Dog.InvalidateVisual(); StateLabel.Text = "●  Connected";
            Headline.Text = "You're in good company.";
            Hint.Text = fullVpn ? "Full-device IPv4 VPN · TCP + UDP · IPv6 blocked" : $"SOCKS5 · 127.0.0.1:{port} · TCP + UDP";
            ConnectButton.Content = "Disconnect";
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { Error.Text = ex.Message; await Disconnect(); }
    }
    async Task Disconnect()
    {
        if (stopping) return; stopping = true;
        session?.Cancel(); connecting = connected = false;
        ConnectButton.IsEnabled = false;
        // EOF asks the VPN guardian to run its finally cleanup before exiting.
        if (vpn is { } v)
        {
            try { v.StandardInput.Close(); await v.WaitForExitAsync().WaitAsync(TimeSpan.FromSeconds(15)); }
            catch (Exception) { Error.Text = "VPN cleanup is still running. Keep namedP open until routes are restored."; }
            if (v.HasExited) { v.Dispose(); vpn = null; }
        }
        if (engine is { } p)
        {
            try { p.StandardInput.Close(); await p.WaitForExitAsync().WaitAsync(TimeSpan.FromSeconds(3)); }
            catch (Exception) { try { p.Kill(true); } catch (InvalidOperationException) { } }
            p.Dispose(); engine = null;
        }
        session?.Dispose(); session = null;
        Dog.State = "idle"; Dog.InvalidateVisual(); StateLabel.Text = "●  Ready when you are";
        Headline.Text = "Your network, a little calmer."; ConnectButton.Content = "Connect →";
        VpnMode.IsEnabled = ProxyMode.IsEnabled = Link.IsEnabled = ConnectButton.IsEnabled = vpn == null;
        UpdateModes(); stopping = false;
    }
    async void OnClosing(object? sender, CancelEventArgs e)
    {
        if (closing) return; e.Cancel = true; await Disconnect();
        if (vpn != null) return;
        closing = true; ticker.Stop(); SystemEvents.UserPreferenceChanged -= SystemThemeChanged; Close();
    }
    static string Bytes(ulong n) => n < 1024 ? $"{n} B" : n < 1048576 ? $"{n / 1024d:F1} KiB" : $"{n / 1048576d:F1} MiB";
}
