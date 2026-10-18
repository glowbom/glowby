// Person mattes for "text behind the presenter" shots, using macOS Vision (on-device, free).
// usage: swift person_matte.swift FRAMES_DIR MATTES_DIR
// Reads every .png in FRAMES_DIR and writes a same-named grayscale .png matte (white = person) at full frame size.
import Foundation
import Vision
import CoreImage
import ImageIO
import UniformTypeIdentifiers

let args = CommandLine.arguments
guard args.count == 3 else { print("usage: swift person_matte.swift FRAMES_DIR MATTES_DIR"); exit(1) }
let inDir = URL(fileURLWithPath: args[1]), outDir = URL(fileURLWithPath: args[2])
try? FileManager.default.createDirectory(at: outDir, withIntermediateDirectories: true)
let ctx = CIContext()
let files = try FileManager.default.contentsOfDirectory(atPath: inDir.path).filter { $0.hasSuffix(".png") }.sorted()
let request = VNGeneratePersonSegmentationRequest()
request.qualityLevel = .accurate
request.outputPixelFormat = kCVPixelFormatType_OneComponent8

for (i, name) in files.enumerated() {
    let url = inDir.appendingPathComponent(name)
    guard let src = CGImageSourceCreateWithURL(url as CFURL, nil), let img = CGImageSourceCreateImageAtIndex(src, 0, nil) else { continue }
    let handler = VNImageRequestHandler(cgImage: img, options: [:])
    try handler.perform([request])
    guard let buffer = request.results?.first?.pixelBuffer else { continue }
    var mask = CIImage(cvPixelBuffer: buffer)
    let sx = CGFloat(img.width) / mask.extent.width, sy = CGFloat(img.height) / mask.extent.height
    mask = mask.transformed(by: CGAffineTransform(scaleX: sx, y: sy))
    guard let out = ctx.createCGImage(mask, from: CGRect(x: 0, y: 0, width: img.width, height: img.height), format: .L8, colorSpace: CGColorSpaceCreateDeviceGray()) else { continue }
    let dest = CGImageDestinationCreateWithURL(outDir.appendingPathComponent(name) as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(dest, out, nil)
    CGImageDestinationFinalize(dest)
    if i % 24 == 0 { print("matte \(i + 1)/\(files.count)") }
}
print("done \(files.count)")
