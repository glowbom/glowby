import 'package:flutter/material.dart';
import 'dart:ui' as ui;

class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    throw UnsupportedError('downloadImage is not supported on this platform.');
  }

  static Future<dynamic> pickImage() async {
    throw UnsupportedError('pickImage is not supported on this platform.');
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
      List<Offset?> points, ui.Image? image, int width, int height) async {
    throw UnsupportedError(
        'convertToBase64JpegWeb is not supported on this platform.');
  }
}
