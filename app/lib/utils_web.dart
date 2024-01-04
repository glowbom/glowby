import 'dart:html' as html;

class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    final windowFeatures =
        'menubar=no,toolbar=no,status=no,resizable=yes,scrollbars=yes,width=600,height=400';
    html.window.open(url, 'glowby-image-${description}', windowFeatures);
  }
}
