import 'dart:convert';
import 'dart:html' as html;
import 'package:flutter/material.dart';

// Uncomment the next line to compile the web version
import 'package:js/js.dart';

// Uncomment the next block to compile the web version

@JS()
external int rv(String lang);

/// Allows assigning a function to be callable from `window.functionName()`
@JS('vr')
external set _vr(void Function(dynamic) f);

/// Allows calling the assigned function from Dart as well.
@JS()
external void vr(text);

class UtilsPlatform {
  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, int width, int height) async {
    // Create a canvas element
    final html.CanvasElement canvas =
        html.CanvasElement(width: width, height: height);
    final html.CanvasRenderingContext2D ctx = canvas.context2D;

    // Set the drawing properties
    ctx.fillStyle = 'white'; // Assuming a white background
    ctx.fillRect(0, 0, width, height); // Fill the canvas with white color
    ctx.strokeStyle = 'black';
    ctx.lineWidth = 2;

    // Draw the lines based on the points
    ctx.beginPath();
    for (int i = 0; i < points.length - 1; i++) {
      if (points[i] != null && points[i + 1] != null) {
        ctx.moveTo(points[i]!.dx, points[i]!.dy);
        ctx.lineTo(points[i + 1]!.dx, points[i + 1]!.dy);
      }
    }
    ctx.stroke();

    // Convert the canvas content to JPEG format
    final String dataUrl =
        canvas.toDataUrl('image/jpeg', 0.9); // 0.9 is the quality

    // Extract the base64 part of the data URL
    final String base64String = dataUrl.split(',')[1];

    return base64String;
  }

  static Future<void> downloadImage(String url, String description) async {
    final windowFeatures =
        'menubar=no,toolbar=no,status=no,resizable=yes,scrollbars=yes,width=600,height=400';
    html.window.open(url, 'glowby-image-${description}', windowFeatures);
  }

  static Future<void> initializeState(dynamic f) {
    _vr = allowInterop(f);
    return Future.value();
  }

  static Future<void> recordVoice(String lang) {
    rv(lang);
    return Future.value();
  }

  static Future<dynamic> startFilePicker() async {
    try {
      html.FileUploadInputElement uploadInput = html.FileUploadInputElement();
      uploadInput.multiple = false;
      uploadInput.accept = '.glowbom';
      dynamic content;

      uploadInput.onChange.listen((e) {
        final files = uploadInput.files;
        if (files != null && files.isNotEmpty) {
          final file = files.first;
          final reader = html.FileReader();

          reader.onLoadEnd.listen((e) {
            String content = reader.result as String;
            content = json.decode(content);
          }).onDone(() {
            return content;
          });

          reader.readAsText(file);
        }
      });

      uploadInput.click();
    } catch (e) {
      print('Error: $e'); // Log the exception
    }

    return null;
  }
}
