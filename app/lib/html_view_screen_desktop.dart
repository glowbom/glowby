// html_view_screen_stub.dart
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:glowby/html_view_screen_interface.dart';
import 'package:path_provider/path_provider.dart';
import 'package:url_launcher/url_launcher_string.dart';

class HtmlViewScreen extends StatelessWidget
    implements HtmlViewScreenInterface {
  final String htmlContent;
  final String appName;

  HtmlViewScreen({required this.htmlContent, required this.appName});

  void _openWebview() async {
    final tempDir = await getTemporaryDirectory();
    final tempFile = File('${tempDir.path}/temp.html');
    await tempFile.writeAsString(htmlContent, flush: true);

    final url = tempFile.uri.toString();
    if (await canLaunchUrlString(url)) {
      await launchUrlString(url);
    } else {
      print("Can't launch $url");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Placeholder for $appName'),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            TextButton(
              onPressed: _openWebview,
              child: Text('Open in Webview'),
            ),
            SizedBox(height: 20),
            SelectableText(htmlContent),
            SizedBox(height: 20), // Spacing between text and button
          ],
        ),
      ),
    );
  }
}
