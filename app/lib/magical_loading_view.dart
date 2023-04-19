import 'dart:math';
import 'package:flutter/material.dart';

class MagicalLoadingView extends StatefulWidget {
  @override
  _MagicalLoadingViewState createState() => _MagicalLoadingViewState();
}

class _MagicalLoadingViewState extends State<MagicalLoadingView>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      duration: const Duration(seconds: 2),
      vsync: this,
    )..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: AnimatedBuilder(
        animation: _controller,
        builder: (context, child) {
          return CustomPaint(
            painter: _MagicalLoadingPainter(_controller.value),
            child: Container(
              width: 200,
              height: 200,
            ),
          );
        },
      ),
    );
  }
}

class _MagicalLoadingPainter extends CustomPainter {
  final double progress;

  _MagicalLoadingPainter(this.progress);

  @override
  void paint(Canvas canvas, Size size) {
    final centerX = size.width / 2;
    final centerY = size.height / 2;
    final radius = min(size.width, size.height) / 4;
    final outerRadius = radius * (1 + 0.3 * sin(2 * pi * progress));

    final circlePaint = Paint()
      ..color = Colors.black.withOpacity(0.5 + 0.5 * sin(2 * pi * progress))
      ..style = PaintingStyle.stroke
      ..strokeWidth = 4;

    canvas.drawCircle(Offset(centerX, centerY), radius, circlePaint);
    canvas.drawCircle(Offset(centerX, centerY), outerRadius, circlePaint);
  }

  @override
  bool shouldRepaint(covariant _MagicalLoadingPainter oldDelegate) {
    return oldDelegate.progress != progress;
  }
}

List<String> loadingMessages = [
  'Generating magical plans...',
  'Talking to planning masterminds...',
  'Breaking down complex tasks...',
  'Creating a manageable plan...',
  'Assembling steps for success...',
  'Crafting an achievable roadmap...',
  'Designing your path to victory...',
  'Simplifying your journey...',
  'Unlocking the secrets of success...',
  'Structuring the plan of action...',
  'Organizing steps for clarity...',
  'Outlining a clear strategy...',
  'Dividing and conquering tasks...',
  'Weaving a plan for triumph...',
  'Mapping your way to accomplishment...',
  'Carving your route to achievement...',
  'Gathering insights for planning...',
  'Turning chaos into order...',
  'Constructing your plan of attack...',
  'Laying the foundation for success...',
  'Distilling the essence of victory...',
  'Collating the blueprint for progress...',
  'Forging a pathway to triumph...',
  'Envisioning a successful future...',
  'Formulating a winning strategy...',
];

String getRandomMessage() {
  final random = Random();
  int index = random.nextInt(loadingMessages.length);
  return loadingMessages[index];
}
