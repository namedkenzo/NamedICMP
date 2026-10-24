param(
 [Parameter(Mandatory=$true)][int]$ParentPid,
 [Parameter(Mandatory=$true)][int]$EnginePid,
 [Parameter(Mandatory=$true)][string]$ServerIP,
 [ValidateRange(1,65535)][int]$Port = 1080
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$parsed = [System.Net.IPAddress]::Parse($ServerIP)
if ($parsed.AddressFamily -ne [System.Net.Sockets.AddressFamily]::InterNetwork) { throw 'IPv4 server required' }
$sessionName = 'namedP-' + [guid]::NewGuid().ToString('N').Substring(0,12)
$routes = @()
$rules = @()
$tun = $null
$adapter = $null
$routeReady = $false
try {
 $exe = Join-Path $PSScriptRoot 'tun2socks.exe'
 if (!(Test-Path $exe) -or !(Test-Path (Join-Path $PSScriptRoot 'wintun.dll'))) { throw 'The release is incomplete: tun2socks.exe or wintun.dll is missing.' }
 $gateway = Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | Where-Object { $_.NextHop -ne '0.0.0.0' } | Sort-Object @{Expression={ $_.RouteMetric + (Get-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4).InterfaceMetric }} | Select-Object -First 1
 if (!$gateway) { throw 'No physical IPv4 default gateway is available.' }
 # Preserve the server route BEFORE adding /1 tunnel routes. Never delete
 # routes that predate this session; retain exact CIM objects for cleanup.
 $existing = @(Get-NetRoute -DestinationPrefix "$ServerIP/32" -ErrorAction SilentlyContinue)
 if ($existing.Count -eq 0) {
  $routes += New-NetRoute -DestinationPrefix "$ServerIP/32" -InterfaceIndex $gateway.InterfaceIndex -NextHop $gateway.NextHop -RouteMetric 1 -PolicyStore ActiveStore
 } elseif (!($existing | Where-Object { $_.InterfaceIndex -eq $gateway.InterfaceIndex -and $_.NextHop -eq $gateway.NextHop })) { throw 'An existing server route conflicts with the physical gateway.' }
 $tun = Start-Process -FilePath $exe -ArgumentList @('-device',"tun://$sessionName",'-proxy',"socks5://127.0.0.1:$Port",'-loglevel','silent') -WindowStyle Hidden -PassThru
 $until = [DateTime]::UtcNow.AddSeconds(15)
 do { Start-Sleep -Milliseconds 150; $tun.Refresh(); if ($tun.HasExited) { throw 'TUN engine stopped before creating an adapter.' }; $adapter = Get-NetAdapter -Name $sessionName -ErrorAction SilentlyContinue } until ($adapter -or [DateTime]::UtcNow -gt $until)
 if (!$adapter) { throw 'Wintun adapter was not created.' }
 $index = $adapter.ifIndex
 Set-NetIPInterface -InterfaceIndex $index -AddressFamily IPv4 -Dhcp Disabled -InterfaceMetric 1 -NlMtuBytes 1280
 New-NetIPAddress -InterfaceIndex $index -IPAddress '198.18.0.1' -PrefixLength 24 -AddressFamily IPv4 -PolicyStore ActiveStore | Out-Null
 Set-DnsClientServerAddress -InterfaceIndex $index -ServerAddresses @('1.1.1.1','1.0.0.1')
 # The ICMP server transport is IPv4. Block IPv6 instead of silently letting
 # it bypass the VPN. Physical-interface DNS is blocked during the session.
 $rules += New-NetFirewallRule -Name "$sessionName-v6" -DisplayName "$sessionName IPv6 guard" -Direction Outbound -Action Block -RemoteAddress '::/0' -Profile Any
 foreach ($physical in @(Get-NetAdapter | Where-Object { $_.ifIndex -ne $index -and $_.Status -eq 'Up' })) {
  foreach ($protocol in @('UDP','TCP')) {
   $rules += New-NetFirewallRule -Name "$sessionName-dns-$($physical.ifIndex)-$protocol" -DisplayName "$sessionName DNS guard" -Direction Outbound -Action Block -Protocol $protocol -RemotePort 53 -InterfaceAlias $physical.Name -Profile Any
  }
 }
 foreach ($prefix in @('0.0.0.0/1','128.0.0.0/1')) { $routes += New-NetRoute -InterfaceIndex $index -DestinationPrefix $prefix -NextHop '0.0.0.0' -RouteMetric 1 -PolicyStore ActiveStore }
 Clear-DnsClientCache
 $routeReady = $true
 [Console]::Out.WriteLine('{"event":"ready"}')
 # Pipe EOF and either process exit both trigger restoration. The guardian
 # outlives a crashed GUI, unlike a cleanup callback in the window itself.
 $closed = [Console]::In.ReadLineAsync()
 while (!$closed.IsCompleted) {
  if (!(Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) -or !(Get-Process -Id $EnginePid -ErrorAction SilentlyContinue)) { break }
  $tun.Refresh(); if ($tun.HasExited) { throw 'TUN engine stopped.' }
  if (!(Get-NetAdapter | Where-Object { $_.ifIndex -eq $gateway.InterfaceIndex -and $_.Status -eq 'Up' })) { throw 'Physical network changed. Reconnect namedP.' }
  Start-Sleep -Milliseconds 500
 }
} catch { [Console]::Error.WriteLine($_.Exception.Message); exit 1 }
finally {
 foreach ($route in $routes) { $route | Remove-NetRoute -Confirm:$false -ErrorAction SilentlyContinue }
 foreach ($rule in $rules) { $rule | Remove-NetFirewallRule -ErrorAction SilentlyContinue }
 if ($adapter) { Set-DnsClientServerAddress -InterfaceIndex $adapter.ifIndex -ResetServerAddresses -ErrorAction SilentlyContinue }
 if ($tun) { if (!$tun.HasExited) { Stop-Process -Id $tun.Id -Force -ErrorAction SilentlyContinue }; $tun.Dispose() }
 Clear-DnsClientCache -ErrorAction SilentlyContinue
}
