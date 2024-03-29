import 'dart:html';
import 'dart:html' as html;
import 'dart:ui_web' as ui;
import 'package:flutter/material.dart';
import 'package:glowby/views/html/html_view_screen_interface.dart';

class HtmlViewScreen extends StatelessWidget
    implements HtmlViewScreenInterface {
  final String htmlContent;
  final String appName;

  const HtmlViewScreen({super.key, required this.htmlContent, required this.appName});

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
      // ignore: unused_local_variable
      final anchor = html.AnchorElement(href: url)
        ..setAttribute("download", "$appName.html")
        ..click();
      html.Url.revokeObjectUrl(url);
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(appName),
        actions: [
          IconButton(
            icon: const Icon(Icons.download, color: Colors.black),
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
