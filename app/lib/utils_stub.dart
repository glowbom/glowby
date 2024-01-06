class UtilsPlatform {
  static Future<void> downloadImage(String url, String description) async {
    throw UnsupportedError('downloadImage is not supported on this platform.');
  }

  static Future<dynamic> startFilePicker() async {
    throw UnsupportedError(
        'startFilePicker is not supported on this platform.');
  }

  static Future<void> initializeState(dynamic f) {
    throw UnsupportedError(
        'initializeState is not supported on this platform.');
  }

  static Future<void> recordVoice(String lang) {
    throw UnsupportedError(
        'initializeState is not supported on this platform.');
  }
}
