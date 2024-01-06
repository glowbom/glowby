import 'dart:io';
import 'package:flutter/material.dart';
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

  static Future<dynamic> startFilePicker() async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> initializeState(dynamic f) async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> recordVoice(String lang) async => throw UnsupportedError(
      'initializeState is not supported on this platform.');

  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, int width, int height) async {
    throw UnsupportedError(
        'convertToBase64JpegWeb is not supported on this platform.');
  }
}
