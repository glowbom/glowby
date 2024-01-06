class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    throw UnsupportedError('downloadImage is not supported on this platform.');
  }

  static Future<dynamic> startFilePicker() async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }
}
