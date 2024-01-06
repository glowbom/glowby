import 'package:flutter/material.dart';

class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    throw UnsupportedError('downloadImage is not supported on this platform.');
  }

  static Future<dynamic> startFilePicker() async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> initializeState(dynamic f) async {
    throw UnsupportedError(
        'initializeState is not supported on this platform.');
  }

  static Future<void> recordVoice(String lang) async {
    throw UnsupportedError(
        'initializeState is not supported on this platform.');
  }

  static Future<String> convertToBase64JpegWeb(
      List<Offset?> points, int width, int height) async {
    throw UnsupportedError(
        'convertToBase64JpegWeb is not supported on this platform.');
  }
}