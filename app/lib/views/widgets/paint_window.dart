import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_image_compress/flutter_image_compress.dart';
import 'package:glowby/views/dialogs/ai_error_dialog.dart';
import 'package:glowby/views/html/html_view_screen.dart';
import 'dart:ui' as ui;

import 'package:glowby/services/openai_api.dart';
import 'package:glowby/utils/utils.dart';
import 'package:flutter/foundation.dart';

class PaintWindow extends StatefulWidget {
  const PaintWindow({super.key});

  @override
  PaintWindowState createState() => PaintWindowState();
}

class PaintWindowState extends State<PaintWindow> {
  final int width = 600;
  final int height = 450;

  List<Offset?> points = [];
  final TextEditingController nameController = TextEditingController();
  String creationName = '';
  bool isLoading = false;
  Uint8List? imgBytes;
  ui.Image? drawingImage;

  @override
  void dispose() {
    nameController.dispose();
    super.dispose();
  }

  // This function converts the drawing (list of points) to a base64 string
  Future<String> convertToBase64JpegMobile(List<Offset?> points) async {
    // Create a picture recorder to record the canvas operations
    final ui.PictureRecorder recorder = ui.PictureRecorder();
    final Canvas canvas = Canvas(recorder);

    // Draw your points here onto the canvas
    final paint = Paint()
      ..color = Colors.black
      ..strokeCap = StrokeCap.round
      ..strokeWidth = 2.0;
    for (int i = 0; i < points.length - 1; i++) {
      if (points[i] != null && points[i + 1] != null) {
        canvas.drawLine(points[i]!, points[i + 1]!, paint);
      }
    }

    // End recording the canvas operations
    final ui.Picture picture = recorder.endRecording();

    // Convert the picture to an image
    final ui.Image image = await picture.toImage(
        width, height); // Set the width and height as needed
    final ByteData? byteData =
        await image.toByteData(format: ui.ImageByteFormat.rawRgba);

    if (byteData == null) {
      if (kDebugMode) {
        print("Failed to obtain byte data from image");
      }
      return '';
    }

    // Compress the image and get JPEG format Uint8List
    final Uint8List imgBytes = await FlutterImageCompress.compressWithList(
      byteData.buffer.asUint8List(),
      minWidth: width,
      minHeight: height,
      quality: 100, // Adjust the quality as needed
      format: CompressFormat.jpeg,
    );

    // Base64 encode the JPEG bytes
    final String base64String = base64Encode(imgBytes);

    return base64String;
  }

  // This function converts the drawing (list of points) to a base64 string
  Future<String> convertToBase64Png(List<Offset?> points) async {
    // Create a picture recorder to record the canvas operations
    final ui.PictureRecorder recorder = ui.PictureRecorder();
    final Canvas canvas = Canvas(recorder);

    // Draw your points here onto the canvas
    final paint = Paint()
      ..color = Colors.black
      ..strokeCap = StrokeCap.round
      ..strokeWidth = 2.0;
    for (int i = 0; i < points.length - 1; i++) {
      if (points[i] != null && points[i + 1] != null) {
        canvas.drawLine(points[i]!, points[i + 1]!, paint);
      }
    }

    // End recording the canvas operations
    final ui.Picture picture = recorder.endRecording();

    // Convert the picture to an image
    final ui.Image image = await picture.toImage(
        width, height); // Set the width and height as needed
    final ByteData? byteData =
        await image.toByteData(format: ui.ImageByteFormat.png);

    // Convert the byte data to a Uint8List
    final Uint8List imgBytes = byteData!.buffer.asUint8List();

    // Base64 encode the image bytes
    final String base64String = base64Encode(imgBytes);

    return base64String;
  }

  Future<void> callOpenAI() async {
    if (isLoading) {
      return;
    }

    setState(() {
      isLoading = true;
    });

    // Convert points to a suitable format and call OpenAI method
    // For example, you might convert points to an image and then to base64
    //String imageBase64 = await convertToBase64Jpeg(points);

    if (drawingImage != null) {
      // Introduce a delay before executing the code
      await Future.delayed(const Duration(milliseconds: 800));
    }
    String imageBase64 =
        await Utils.convertToBase64JpegWeb(points, drawingImage, width, height);

    // this is for testing
    // imgBytes = base64Decode(imageBase64); // Implement this function

    String htmlResponse =
        await OpenAiApi().getHtmlFromOpenAI(imageBase64, creationName);

    String htmlContent = creationName;

    if (htmlResponse == '') {
      if (mounted) {
        Navigator.of(context).pop();
      }

      _showAiErrorDialog();
      return;
    }

    try {
      htmlContent = htmlResponse.split("```html")[1].split('```')[0];
    } catch (e) {
      htmlContent = htmlResponse;
    }

    // Use the captured context after the async gap
    if (mounted) {
      Navigator.push(
        context, // use the safeContext that was captured before the async gap
        MaterialPageRoute(
          builder: (context) => HtmlViewScreen(
            htmlContent: htmlContent,
            appName: creationName,
          ),
        ),
      );
    }

    setState(() {
      isLoading = false;
    });

    clear();
  }

  void _showAiErrorDialog() {
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return const AiErrorDialog();
      },
    ).then(
      (value) => setState(() {}),
    );
  }

  void clear() {
    drawingImage = null;
    nameController.clear();
    setState(() {
      points.clear();
    });
  }

  Future<void> loadImage(Uint8List imageBytes) async {
    final Completer<ui.Image> completer = Completer<ui.Image>();
    ui.decodeImageFromList(imageBytes, (ui.Image img) {
      if (!completer.isCompleted) {
        completer.complete(img);
      }
    });
    drawingImage = await completer.future;
    setState(() {}); // Trigger a repaint
  }

  Future<void> uploadImage() async {
    try {
      final Uint8List? value = await Utils.pickImage();
      if (value != null) {
        await loadImage(value); // Load the image and update the state
      }
    } catch (e) {
      // Handle the error or display an error message
      if (kDebugMode) {
        print('Error picking image: $e');
      }
    }

    /*final picker = ImagePicker();
    final pickedFile = await picker.getImage(source: ImageSource.gallery);

    if (pickedFile != null) {
      // If the pickedFile is not null, then we have a path to the image file.
      // You can now upload this image to a server or use it in your application.
      final File imageFile = File(pickedFile.path);
      // Implement your image upload functionality here
    } else {
      print('No image selected.');
    }*/
  }

  @override
  Widget build(BuildContext context) {
    if (imgBytes != null) {
      return Image.memory(imgBytes!);
    }

    return AlertDialog(
      title: const Text('Magic Window (Powered by GPT-4 with Vision)'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: width.toDouble(),
              height: height.toDouble(),
              decoration: BoxDecoration(
                border: Border.all(color: Colors.black),
                color: Colors.white,
              ),
              child: GestureDetector(
                onPanUpdate: (DragUpdateDetails details) {
                  setState(() {
                    RenderBox renderBox =
                        context.findRenderObject() as RenderBox;
                    points.add(renderBox.globalToLocal(details.localPosition));
                  });
                },
                onPanEnd: (DragEndDetails details) {
                  setState(() {
                    points.add(
                        null); // Add a null to the list to separate the lines
                  });
                },
                child: CustomPaint(
                  painter: DrawingPainter(points: points, image: drawingImage),
                  size: Size.infinite,
                ),
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(8.0),
              child: SizedBox(
                width: width.toDouble(), // Set your desired maximum width here
                child: TextField(
                  controller: nameController,
                  decoration: const InputDecoration(
                    labelText: 'Name your creation',
                  ),
                  onChanged: (value) {
                    creationName = value;
                  },
                ),
              ),
            ),
            Row(
              mainAxisAlignment: MainAxisAlignment.start,
              children: [
                IconButton(
                  icon: const Icon(Icons.image),
                  onPressed: uploadImage,
                ),
              ],
            ),
          ],
        ),
      ),
      actions: <Widget>[
        if (isLoading && drawingImage != null)
          const Text('Loading...')
        else if (isLoading)
          const CircularProgressIndicator()
        else
          TextButton(
            onPressed: clear,
            child: const Text('Clear'),
          ),
        TextButton(
          onPressed: callOpenAI,
          child: const Text(
              'Build'), // Here we call the method to process the drawing
        ),
        TextButton(
          child: const Text('Close'),
          onPressed: () {
            Navigator.of(context).pop();
          },
        ),
      ],
    );
  }
}

class DrawingPainter extends CustomPainter {
  final List<Offset?> points;
  final ui.Image? image;

  DrawingPainter({required this.points, this.image});

  void paintImage(
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

  @override
  void paint(Canvas canvas, Size size) {
    // If there's an image, draw it
    if (image != null) {
      paintImage(canvas: canvas, image: image!, size: size);
    }

    var paint = Paint()
      ..color = Colors.black
      ..strokeCap = StrokeCap.round
      ..strokeWidth = 2.0;

    for (int i = 0; i < points.length - 1; i++) {
      if (points[i] != null && points[i + 1] != null) {
        // Check if both points are within the bounds of the CustomPaint widget
        if (points[i]!.dx >= 0 &&
            points[i]!.dx <= size.width &&
            points[i]!.dy >= 0 &&
            points[i]!.dy <= size.height &&
            points[i + 1]!.dx >= 0 &&
            points[i + 1]!.dx <= size.width &&
            points[i + 1]!.dy >= 0 &&
            points[i + 1]!.dy <= size.height) {
          canvas.drawLine(points[i]!, points[i + 1]!, paint);
        }
      }
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => true;
}
