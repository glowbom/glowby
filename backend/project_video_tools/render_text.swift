// Renders one line of text as a transparent PNG in the Glowbom style: SF (system font) at weight 750,
// tight tracking, and a soft drop shadow. White by default; --ink black for bright backgrounds.
// usage: swift render_text.swift "TEXT" OUT.png [--size 160] [--ink white|black] [--tracking -0.05]
import AppKit
import CoreText

var args = Array(CommandLine.arguments.dropFirst())
guard args.count >= 2 else { print("usage: swift render_text.swift \"TEXT\" OUT.png [--size 160] [--ink white|black] [--tracking -0.05]"); exit(1) }
let text = args.removeFirst(), out = args.removeFirst()
var size: CGFloat = 160, ink = "white", tracking: CGFloat = -0.05
while args.count >= 2 {
    let key = args.removeFirst(), value = args.removeFirst()
    switch key {
    case "--size": size = CGFloat(Double(value) ?? 160)
    case "--ink": ink = value
    case "--tracking": tracking = CGFloat(Double(value) ?? -0.05)
    default: break
    }
}
// Weight 750 sits between bold (0.4) and heavy (0.56) on AppKit's scale.
let font = NSFont.systemFont(ofSize: size, weight: NSFont.Weight(rawValue: 0.48))
let black = ink == "black"
let color = black ? NSColor(srgbRed: 0x11 / 255.0, green: 0x11 / 255.0, blue: 0x11 / 255.0, alpha: 1) : NSColor.white
let attributes: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: color, .kern: tracking * size]
let line = CTLineCreateWithAttributedString(NSAttributedString(string: text, attributes: attributes))
let bounds = CTLineGetBoundsWithOptions(line, [.useGlyphPathBounds])
// Room for the shadow on every side.
let pad = size * 0.35
let width = Int(ceil(bounds.width + pad * 2)), height = Int(ceil(bounds.height + pad * 2))
guard let ctx = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                          space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { exit(1) }
let origin = CGPoint(x: pad - bounds.minX, y: pad - bounds.minY)
// Two soft shadows, as in the CSS house style: a close one and a wide one (y grows downward in CSS).
let scale = size / 210
let shadows: [(CGFloat, CGFloat, CGFloat)] = black ? [(4, 6, 0.20), (16, 34, 0.26)] : [(3, 5, 0.30), (14, 32, 0.38)]
for (offset, blur, alpha) in shadows {
    ctx.saveGState()
    ctx.setShadow(offset: CGSize(width: 0, height: -offset * scale), blur: blur * scale, color: NSColor(white: 0, alpha: alpha).cgColor)
    ctx.textPosition = origin
    CTLineDraw(line, ctx)
    ctx.restoreGState()
}
ctx.textPosition = origin
CTLineDraw(line, ctx)
guard let image = ctx.makeImage(), let dest = CGImageDestinationCreateWithURL(URL(fileURLWithPath: out) as CFURL, "public.png" as CFString, 1, nil) else { exit(1) }
CGImageDestinationAddImage(dest, image, nil)
CGImageDestinationFinalize(dest)
print("\(out) \(width)x\(height)")
