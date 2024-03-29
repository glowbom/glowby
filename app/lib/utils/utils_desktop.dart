import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
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

  static Future<dynamic> pickImage() async {
    if (Platform.isIOS) {
      final picker = ImagePicker();
      final pickedFile = await picker.pickImage(source: ImageSource.gallery);
      if (pickedFile != null) {
        final File imageFile = File(pickedFile.path);
        return imageFile.readAsBytes();
      } else {
        if (kDebugMode) {
          print('No image selected.');
        }
        return null;
      }
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

  static void paintImage(
      {required Canvas canvas, required ui.Image image, required Size size}) {
    // Calculate the scale factor to fit the image within the canvas if needed
    final double scaleFactor =
        min(size.width / image.width, size.height / image.height);

    // Calculate the destination rectangle for the scaled image
    final Rect destRect = Rect.fromLTWH(
      (size.width - image.width * scaleFactor) / 2,
      (size.height - image.height * scaleFactor) / 2,
      image.width * scaleFactor,
      image.height * scaleFactor,
    );

    // Draw the scaled image at the center position
    canvas.drawImageRect(
        image,
        Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
        destRect,
        Paint());
  }

  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, ui.Image? img, int width, int height) async {
    // Create a PictureRecorder to record the canvas operations
    final ui.PictureRecorder recorder = ui.PictureRecorder();
    final ui.Canvas canvas = ui.Canvas(recorder);

    // Fill the canvas with a white background
    final ui.Paint paintBackground = ui.Paint()
      ..color = const ui.Color(0xFFFFFFFF);
    canvas.drawRect(ui.Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
        paintBackground);

    // If there's an image, draw it
    if (img != null) {
      final size = Size(width.toDouble(), height.toDouble());
      paintImage(canvas: canvas, image: img, size: size);
    }

    // Draw the lines with a black paint
    final ui.Paint paintLines = ui.Paint()
      ..color = const ui.Color(0xFF000000)
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
