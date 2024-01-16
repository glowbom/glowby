import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

class HtmlViewScreenMobile extends StatefulWidget {
  final String htmlContent;
  final String appName;

  HtmlViewScreenMobile({required this.htmlContent, required this.appName});

  @override
  _HtmlViewScreenState createState() => _HtmlViewScreenState();
}

class _HtmlViewScreenState extends State<HtmlViewScreenMobile> {
  late final WebViewController _controller;

  @override
  void initState() {
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..loadHtmlString(widget.htmlContent);
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('Placeholder for ${widget.appName}'),
      ),
      body: WebViewWidget(controller: _controller),
    );
  }
}
