import 'dart:convert';

import 'package:flutter/material.dart';
import 'dart:ui' as ui;
import 'dart:typed_data';

class PaintWindow extends StatefulWidget {
  @override
  _PaintWindowState createState() => _PaintWindowState();
}

class _PaintWindowState extends State<PaintWindow> {
  List<Offset?> points = [];
  final TextEditingController nameController = TextEditingController();
  String creationName = '';
  bool isLoading = false;

  @override
  void dispose() {
    nameController.dispose();
    super.dispose();
  }

  // This function converts the drawing (list of points) to a base64 string
  Future<String> convertToBase64(List<Offset?> points) async {
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
    final ui.Image image =
        await picture.toImage(300, 300); // Set the width and height as needed
    final ByteData? byteData =
        await image.toByteData(format: ui.ImageByteFormat.png);

    // Convert the byte data to a Uint8List
    final Uint8List imgBytes = byteData!.buffer.asUint8List();

    // Base64 encode the image bytes
    final String base64String = base64Encode(imgBytes);

    return base64String;
  }

  Future<void> callOpenAI() async {
    setState(() {
      isLoading = true;
    });
    // Convert points to a suitable format and call OpenAI method
    // For example, you might convert points to an image and then to base64
    String imageBase64 =
        await convertToBase64(points); // Implement this function
    print(imageBase64);
    // String htmlResponse = await OpenAI_API().getHtmlFromOpenAI(imageBase64, creationName);

    // Do something with htmlResponse

    setState(() {
      isLoading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Paint Window'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: double.infinity,
              height: 300,
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
                  painter: DrawingPainter(points: points),
                  size: Size.infinite,
                ),
              ),
            ),
            Padding(
              padding: EdgeInsets.all(8.0),
              child: TextField(
                controller: nameController,
                decoration: InputDecoration(
                  labelText: 'Name your creation',
                ),
                onChanged: (value) {
                  creationName = value;
                },
              ),
            ),
          ],
        ),
      ),
      actions: <Widget>[
        if (isLoading)
          CircularProgressIndicator()
        else
          TextButton(
            child: const Text('Clear'),
            onPressed: () {
              setState(() {
                points.clear();
              });
            },
          ),
        TextButton(
          child: const Text('Build'),
          onPressed:
              callOpenAI, // Here we call the method to process the drawing
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

  DrawingPainter({required this.points});

  @override
  void paint(Canvas canvas, Size size) {
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
