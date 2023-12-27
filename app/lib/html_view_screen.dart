import 'dart:html';
import 'dart:ui_web' as ui;
import 'package:flutter/material.dart';

class HtmlViewScreen extends StatelessWidget {
  final String htmlContent;
  final String appName;

  HtmlViewScreen({required this.htmlContent, required this.appName});

  @override
  Widget build(BuildContext context) {
    // Generate a unique key based on the html content.
    String contentKey = DateTime.now().millisecondsSinceEpoch.toString();

    // Registers the web view with a unique key
    ui.platformViewRegistry.registerViewFactory(
      contentKey,
      (int viewId) {
        IFrameElement iframeElement = IFrameElement()
          ..style.border = 'none'
          ..style.height = '100%'
          ..style.width = '100%'
          ..srcdoc = htmlContent;
        return iframeElement;
      },
    );

    return Scaffold(
      appBar: AppBar(
        title: Text(appName),
      ),
      body: HtmlElementView(
        key: ValueKey(contentKey), // Use the unique key for the HtmlElementView
        viewType: contentKey,
      ),
    );
  }
}
