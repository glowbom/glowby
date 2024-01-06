// html_view_screen_stub.dart
import 'package:flutter/material.dart';
import 'package:glowby/html_view_screen_interface.dart';

class HtmlViewScreen extends StatelessWidget
    implements HtmlViewScreenInterface {
  final String htmlContent;
  final String appName;

  HtmlViewScreen({required this.htmlContent, required this.appName});

  @override
  Widget build(BuildContext context) {
    // For desktop, return an alternative widget or a placeholder
    return Scaffold(
      appBar: AppBar(
        title: Text('Placeholder for $appName'),
      ),
      body: Center(
        child: Text('HTML content is not viewable in the desktop application.'),
      ),
    );
  }
}
