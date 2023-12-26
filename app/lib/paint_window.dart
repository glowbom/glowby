import 'package:flutter/material.dart';

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

  Future<void> callOpenAI() async {
    setState(() {
      isLoading = true;
    });
    // Convert points to a suitable format and call OpenAI method
    // For example, you might convert points to an image and then to base64
    // String imageBase64 = convertToBase64(points); // Implement this function
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
