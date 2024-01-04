import 'dart:io';
import 'package:path_provider/path_provider.dart';
import 'package:http/http.dart' as http;

class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    // Desktop-specific image downloading implementation
    final response = await http.get(Uri.parse(url));
    if (response.statusCode == 200) {
      final directory =
          await getDownloadsDirectory(); // path_provider package needed
      if (directory != null) {
        final filePath = '${directory.path}/$description.png';
        final file = File(filePath);
        await file.writeAsBytes(response.bodyBytes);
      }
    } else {
      throw Exception(
          'Failed to download image: Server responded with status code ${response.statusCode}');
    }
  }
}
