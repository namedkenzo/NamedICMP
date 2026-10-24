using System.Diagnostics;
using System.Windows;
using System.Windows.Media;

namespace NamedP;

// Original vector artwork: resolution independent and no animation assets.
public sealed class Dachshund : FrameworkElement
{
    public string State { get; set; } = "idle";
    readonly Stopwatch clock = Stopwatch.StartNew();
    readonly EventHandler render;
    public Dachshund()
    {
        render = (_, _) => { if (IsVisible && SystemParameters.ClientAreaAnimation) InvalidateVisual(); };
        Loaded += (_, _) => CompositionTarget.Rendering += render;
        Unloaded += (_, _) => CompositionTarget.Rendering -= render;
    }
    static Brush B(string hex) => new SolidColorBrush((Color)ColorConverter.ConvertFromString(hex));
    static void Shape(DrawingContext d, string path, Brush fill, Pen? stroke = null) => d.DrawGeometry(fill, stroke, Geometry.Parse(path));
    protected override void OnRender(DrawingContext d)
    {
        base.OnRender(d);
        double t = SystemParameters.ClientAreaAnimation ? clock.Elapsed.TotalSeconds : 0;
        double scale = Math.Min(ActualWidth / 420, ActualHeight / 250);
        d.PushTransform(new TranslateTransform((ActualWidth - 420 * scale) / 2, (ActualHeight - 250 * scale) / 2));
        d.PushTransform(new ScaleTransform(scale, scale));
        var coat = B("#AD6338"); var shade = B("#713D2C"); var gold = B("#D59A61");
        d.DrawEllipse(B("#18302D"), null, new Point(210, 204), 132, 9);
        double bounce = State == "connecting" ? Math.Sin(t * 11) * 4 : Math.Sin(t * 2) * 1.5;
        d.PushTransform(new TranslateTransform(0, bounce));
        // A tiny forward/back run while connecting, not a spinning window.
        d.PushTransform(new TranslateTransform(State == "connecting" ? Math.Sin(t * 3) * 12 : 0, 0));
        d.PushTransform(new RotateTransform(State == "connected" ? Math.Sin(t * 17) * 26 : Math.Sin(t * 2) * 4, 103, 136));
        Shape(d, "M105,141 C75,137 55,112 59,94 C64,100 72,119 108,123 Z", coat);
        d.Pop();
        double step = State == "connecting" ? Math.Sin(t * 11) * 13 : 0;
        d.PushTransform(new RotateTransform(step, 134, 159));
        d.DrawRoundedRectangle(shade, null, new Rect(123, 151, 18, 48), 8, 8); d.Pop();
        d.PushTransform(new RotateTransform(-step, 289, 159));
        d.DrawRoundedRectangle(shade, null, new Rect(281, 153, 18, 46), 8, 8); d.Pop();
        Shape(d, "M101,133 C108,112 135,111 160,114 L270,114 C301,104 322,124 317,148 C315,173 294,178 270,177 L143,177 C112,177 94,159 101,133 Z", coat);
        Shape(d, "M126,145 C154,145 232,143 282,143 C297,143 299,167 280,170 L146,170 C127,170 120,156 126,145 Z", gold);
        d.PushTransform(new RotateTransform(-step, 153, 162));
        d.DrawRoundedRectangle(coat, null, new Rect(145, 157, 19, 44), 8, 8);
        d.DrawRoundedRectangle(coat, null, new Rect(142, 192, 29, 10), 5, 5); d.Pop();
        d.PushTransform(new RotateTransform(step, 304, 161));
        d.DrawRoundedRectangle(coat, null, new Rect(298, 154, 19, 47), 8, 8);
        d.DrawRoundedRectangle(coat, null, new Rect(296, 192, 29, 10), 5, 5); d.Pop();
        d.PushTransform(new RotateTransform(State == "connected" ? -5 : Math.Sin(t * 1.7) * 2, 308, 126));
        Shape(d, "M282,121 C267,83 284,58 311,59 C337,59 346,81 344,99 L365,104 C381,105 384,122 372,130 L330,137 C306,145 290,136 282,121 Z", coat);
        Shape(d, "M299,74 C277,67 271,92 279,119 C282,133 291,147 303,137 C314,122 316,86 299,74 Z", shade);
        Shape(d, "M332,111 C343,113 357,114 371,112 L371,127 L338,132 Z", gold);
        d.DrawEllipse(B("#241F21"), null, new Point(371, 111), 10, 7);
        bool blink = t % 5.4 > 5.2;
        d.DrawEllipse(B("#241F21"), null, new Point(328, 89), 5, blink ? 0.8 : 6);
        if (!blink) d.DrawEllipse(Brushes.White, null, new Point(329.5, 87), 1.8, 1.8);
        if (State == "connected") Shape(d, "M345,129 Q353,140 361,128", Brushes.Transparent, new Pen(shade, 2));
        d.Pop();
        Shape(d, "M287,134 Q298,142 316,136 L318,144 Q300,151 285,142 Z", B("#63C7B0"));
        d.DrawEllipse(B("#EBC57F"), null, new Point(307, 149), 4, 5);
        Shape(d, "M132,122 Q192,118 255,122", Brushes.Transparent, new Pen(B("#CA8953"), 3));
        d.Pop(); d.Pop(); d.Pop(); d.Pop();
    }
}
