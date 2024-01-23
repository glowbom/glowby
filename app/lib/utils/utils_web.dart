import 'dart:async';
import 'dart:convert';
import 'dart:ui' as ui;
import 'dart:html' as html;
import 'package:flutter/foundation.dart';
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
  static Future<void> drawImageOnCanvas(html.CanvasRenderingContext2D ctx,
      ui.Image image, double width, double height) async {
    Completer<void> completer = Completer<void>();

    // Convert the ui.Image to a base64 string
    final c = Completer<ByteData>();
    image.toByteData().then((byteData) {
      c.complete(byteData);
    });
    final byteData = await c.future;

    Uint8List uint8list = byteData.buffer.asUint8List();
    String base64Str = base64Encode(uint8list);

    // Create the ImageElement with the base64 string as the source
    html.ImageElement imgElement = html.ImageElement();
    imgElement.src = 'data:image/png;base64,$base64Str';

    imgElement.onLoad.listen((event) {
      // Draw the image on the canvas when it's loaded
      ctx.drawImageScaledFromSource(
          imgElement,
          0, // source x
          0, // source y
          image.width.toDouble(), // source width
          image.height.toDouble(), // source height
          0, // destination x
          0, // destination y
          width, // destination width
          height // destination height
          );
      completer.complete();
    });

    return completer.future;
  }

  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, ui.Image? image, int width, int height) async {
    // Create a canvas element
    final html.CanvasElement canvas =
        html.CanvasElement(width: width, height: height);
    final html.CanvasRenderingContext2D ctx = canvas.context2D;

    // Set the drawing properties
    ctx.fillStyle = 'white'; // Assuming a white background
    ctx.fillRect(0, 0, width, height); // Fill the canvas with white color
    ctx.strokeStyle = 'black';
    ctx.lineWidth = 2;

    if (image != null) {
      await drawImageOnCanvas(ctx, image, width.toDouble(), height.toDouble());
    }

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
    const windowFeatures =
        'menubar=no,toolbar=no,status=no,resizable=yes,scrollbars=yes,width=600,height=400';
    html.window.open(url, 'glowby-image-$description', windowFeatures);
  }

  static Future<void> initializeState(dynamic f) {
    _vr = allowInterop(f);
    return Future.value();
  }

  static Future<void> recordVoice(String lang) {
    rv(lang);
    return Future.value();
  }

  static Future<dynamic> pickImage() async {
    // Create an input element for file upload
    final uploadInput = html.FileUploadInputElement();
    uploadInput.accept = 'image/*'; // Accept only image files

    // Trigger the file picker dialog
    uploadInput.click();

    // Wait for the user to select a file
    await uploadInput.onChange.first;

    // Get the selected file
    final file = uploadInput.files!.first;

    // Read the file as data URL
    final reader = html.FileReader();
    reader.readAsDataUrl(file);

    // Wait for the file to be read
    await reader.onLoadEnd.first;

    // Return the result
    return reader.result;
  }

  static Future<dynamic> startFilePicker() async {
    Completer completer = Completer<dynamic>();
    try {
      html.FileUploadInputElement uploadInput = html.FileUploadInputElement();
      uploadInput.multiple = false;
      uploadInput.accept = '.glowbom';

      uploadInput.onChange.listen((e) {
        final files = uploadInput.files;
        if (files != null && files.isNotEmpty) {
          final file = files.first;
          final reader = html.FileReader();

          reader.onLoadEnd.listen((e) {
            dynamic result = reader.result;
            if (result is String) {
              completer.complete(json.decode(result));
            } else {
              completer.complete(
                  result); // Assuming result is already in the correct format
            }
          });

          reader.readAsText(file);
        } else {
          completer.complete(null);
        }
      });

      uploadInput.click();
    } catch (e) {
      if (kDebugMode) {
        print('Error: $e'); // Log the exception
      } // Log the exception
      completer.completeError(e);
    }

    return completer.future;
  }
}
