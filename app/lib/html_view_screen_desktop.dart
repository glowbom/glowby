// html_view_screen_stub.dart
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_highlight/flutter_highlight.dart';
import 'package:flutter_highlight/themes/github.dart';
import 'package:glowby/html_view_screen_interface.dart';
import 'package:glowby/html_view_screen_mobile.dart';
import 'package:path_provider/path_provider.dart';
import 'package:url_launcher/url_launcher_string.dart';

class HtmlViewScreen extends StatelessWidget
    implements HtmlViewScreenInterface {
  final String htmlContent;
  final String appName;

  HtmlViewScreen({required this.htmlContent, required this.appName});

  void _openCodeInBrowser(context) async {
    if (Platform.isIOS) {
      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (context) => HtmlViewScreenMobile(
            htmlContent: htmlContent,
            appName: appName,
          ),
        ),
      );
    } else {
      // Use default implementation
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
  }

  @override
  Widget build(BuildContext context) {
    //_openCodeInBrowser(context);

    return Scaffold(
      appBar: AppBar(
        title: Text('Placeholder for $appName'),
      ),
      body: Container(
        alignment: Alignment.center,
        child: SingleChildScrollView(
          // Makes the content scrollable
          padding: EdgeInsets.all(16), // Adds padding around the edges
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              TextButton(
                onPressed: () => _openCodeInBrowser(context),
                child: Text('Run code'),
              ),
              SizedBox(height: 20), // Spacing between button and code viewer
              // Code viewer for HTML content
              HighlightView(
                htmlContent,
                language: 'html',
                theme: githubTheme, // Choose the theme you like
                padding:
                    EdgeInsets.all(12), // Adds padding inside the code viewer
                textStyle: TextStyle(fontFamily: 'monospace', fontSize: 10.0),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
