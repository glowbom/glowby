import 'dart:html';
import 'dart:html' as html;
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

    // Function to download the content
    void downloadContent() {
      final blob = html.Blob([htmlContent]);
      final url = html.Url.createObjectUrlFromBlob(blob);
      final anchor = html.AnchorElement(href: url)
        ..setAttribute("download", "${appName}.html")
        ..click();
      html.Url.revokeObjectUrl(url);
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(appName),
        actions: [
          IconButton(
            icon: Icon(Icons.download),
            onPressed: downloadContent, // Trigger the download
            tooltip: 'Download Code',
          ),
        ],
      ),
      body: HtmlElementView(
        key: ValueKey(contentKey), // Use the unique key for the HtmlElementView
        viewType: contentKey,
      ),
    );
  }
}
