import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;
import 'dart:html' as html;
import 'dart:math' as math;
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
  static Future<html.ImageElement> convertUiImageToHtmlImage(
      ui.Image image) async {
    final byteData = await image.toByteData(format: ui.ImageByteFormat.png);

    if (byteData == null) {
      throw Exception('Failed to convert ui.Image to ByteData');
    }

    final Uint8List uint8list = byteData.buffer.asUint8List();
    final base64Str = base64Encode(uint8list);
    final imgElement = html.ImageElement();
    imgElement.src = 'data:image/png;base64,$base64Str';
    return imgElement;
  }

  static Future<Uint8List> convertHtmlImageToUint8List(
      html.ImageElement imageElement) async {
    final response = await html.HttpRequest.request(
      imageElement.src!,
      method: "GET",
      responseType: "arraybuffer",
    );

    return Uint8List.view(response.response as ByteBuffer);
  }

  static Future<void> drawImageOnCanvas(html.CanvasRenderingContext2D ctx,
      ui.Image image, double canvasWidth, double canvasHeight) async {
    Completer<void> completer = Completer<void>();

    final imgElement = await convertUiImageToHtmlImage(image);

    imgElement.onLoad.listen((event) {
      // Calculate the scaling factor to maintain aspect ratio
      double scaleX = canvasWidth / image.width;
      double scaleY = canvasHeight / image.height;
      double scale = math.min(scaleX, scaleY);

      // Calculate the centered position
      double centeredX = (canvasWidth - (image.width * scale)) / 2;
      double centeredY = (canvasHeight - (image.height * scale)) / 2;

      // Draw the image on the canvas with the correct scaling and centered
      ctx.drawImageScaledFromSource(
          imgElement,
          0, // source x
          0, // source y
          image.width.toDouble(), // source width
          image.height.toDouble(), // source height
          centeredX, // destination x
          centeredY, // destination y
          image.width.toDouble() * scale, // destination width
          image.height.toDouble() * scale // destination height
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

  static Future<Uint8List> pickImage() async {
    final uploadInput = html.FileUploadInputElement();
    uploadInput.accept = 'image/*';
    uploadInput.click();

    await uploadInput.onChange.first;

    final file = uploadInput.files!.first;
    final reader = html.FileReader();
    reader.readAsArrayBuffer(file);

    await reader.onLoadEnd.first;

    return Uint8List.fromList(reader.result as List<int>);
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

  static Future<dynamic> downloadSourceCode() async {
    Completer completer = Completer<dynamic>();

    String fileName = "chat-pulze.zip";
    if (kDebugMode) {
      // Append .zip extension
      print("Downloading file as: $fileName");
    } // Print the filename in the console

    html.AnchorElement(
        href:
            'https://github.com/glowbom/glowby/releases/download/2.6/chat-multion.zip')
      ..download = fileName
      ..click();

    return completer.future;
  }
}
