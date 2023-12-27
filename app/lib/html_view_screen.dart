import 'dart:html';
import 'dart:ui_web' as ui;

import 'package:flutter/material.dart';

class HtmlViewScreen extends StatelessWidget {
  final String htmlContent;
  final String appName;

  HtmlViewScreen({required this.htmlContent, required this.appName});

  // Unique key for the IFrame
  final String iframeKey = 'iframe-key';

  @override
  Widget build(BuildContext context) {
    // Avoids UI rebuild from losing state
    ui.platformViewRegistry.registerViewFactory(
      iframeKey,
      (int viewId) {
        final IFrameElement iframeElement = IFrameElement()
          ..style.border = 'none' // Removes the iframe border
          ..style.height = '100%'
          ..style.width = '100%'
          ..srcdoc = htmlContent; // Sets the HTML content directly

        return iframeElement;
      },
    );

    return Scaffold(
      appBar: AppBar(
        title: Text(appName),
      ),
      body: HtmlElementView(
        viewType: iframeKey,
      ),
    );
  }
}
