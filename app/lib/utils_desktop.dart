import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:http/http.dart' as http;
import 'dart:ui' as ui;

class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    // Desktop-specific image downloading implementation
    final response = await http.get(Uri.parse(url));
    if (response.statusCode == 200) {
      final directory =
          await getDownloadsDirectory(); // path_provider package needed
      if (directory != null) {
        final filePath = '${directory.path}/$description.png';
        final file = File(filePath);
        await file.writeAsBytes(response.bodyBytes);
      }
    } else {
      throw Exception(
          'Failed to download image: Server responded with status code ${response.statusCode}');
    }
  }

  static Future<dynamic> startFilePicker() async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> initializeState(dynamic f) async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> recordVoice(String lang) async => throw UnsupportedError(
      'initializeState is not supported on this platform.');

  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, int width, int height) async {
    // Create a PictureRecorder to record the canvas operations
    final ui.PictureRecorder recorder = ui.PictureRecorder();
    final ui.Canvas canvas = ui.Canvas(recorder);

    // Fill the canvas with a white background
    final ui.Paint paintBackground = ui.Paint()..color = ui.Color(0xFFFFFFFF);
    canvas.drawRect(ui.Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
        paintBackground);

    // Draw the lines with a black paint
    final ui.Paint paintLines = ui.Paint()
      ..color = ui.Color(0xFF000000)
      ..strokeWidth = 2.0;

    for (int i = 0; i < points.length - 1; i++) {
      if (points[i] != null && points[i + 1] != null) {
        canvas.drawLine(points[i]!, points[i + 1]!, paintLines);
      }
    }

    // Convert the canvas into a Picture
    final ui.Picture picture = recorder.endRecording();

    // Then convert the Picture into an Image
    final ui.Image image = await picture.toImage(width, height);

    // Encode the image to a ByteData as a PNG
    final ByteData? byteData =
        await image.toByteData(format: ui.ImageByteFormat.png);
    final Uint8List pngBytes = byteData!.buffer.asUint8List();

    // Finally, encode the PNG bytes as a base64 string
    final String base64String = base64Encode(pngBytes);

    return base64String;
  }
}
