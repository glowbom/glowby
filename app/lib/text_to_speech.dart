import 'package:flutter_tts/flutter_tts.dart';

class TextToSpeech {
  FlutterTts? _flutterTts;

  static final Map<String, String> _languageCodes = {
    'American English': 'en-US',
    'American Spanish': 'es-US',
    'Arabic (Saudi Arabia)': 'ar-SA',
    'Argentinian Spanish': 'es-AR',
    'Australian English': 'en-AU',
    'Brazilian Portuguese': 'pt-BR',
    'British English': 'en-GB',
    'Bulgarian': 'bg-BG',
    'Canadian French': 'fr-CA',
    'Chinese (Simplified)': 'zh-CN',
    'Chinese (Traditional)': 'zh-TW',
    'Czech': 'cs-CZ',
    'Danish': 'da-DK',
    'Dutch': 'nl-NL',
    'English': 'en-US',
    'Finnish': 'fi-FI',
    'French': 'fr-FR',
    'German': 'de-DE',
    'Greek': 'el-GR',
    'Hebrew (Israel)': 'he-IL',
    'Hungarian': 'hu-HU',
    'Indonesian': 'id-ID',
    'Italian': 'it-IT',
    'Japanese': 'ja-JP',
    'Korean': 'ko-KR',
    'Mexican Spanish': 'es-MX',
    'Norwegian': 'nb-NO',
    'Polish': 'pl-PL',
    'Portuguese': 'pt-PT',
    'Romanian': 'ro-RO',
    'Russian': 'ru-RU',
    'Slovak': 'sk-SK',
    'Spanish': 'es-ES',
    'Swedish': 'sv-SE',
    'Thai': 'th-TH',
    'Turkish': 'tr-TR',
    'Ukrainian': 'uk-UA',
    'Vietnamese': 'vi-VN',
  };

  static Map<String, String> get languageCodes => _languageCodes;
  static String? lastLanguage = null;

  Future<void> speakText(String text, {String language = 'en-US'}) async {
    if (text == 'typing...') {
      return;
    }

    if (_flutterTts == null) {
      _flutterTts = FlutterTts();
    }

    // Split the text into lines
    List<String> lines = text.split('\n');

    // Process each line separately
    for (String line in lines) {
      String currentLanguage = language;

      // Check if a language code is present in the line
      String languageName = '';
      for (final entry in _languageCodes.entries) {
        if (line.contains('${entry.key}: ')) {
          currentLanguage = entry.value;
          languageName = entry.key;
          line = line.replaceAll('${entry.key}: ', '');
          break;
        }
      }

      if (currentLanguage.isNotEmpty && currentLanguage != lastLanguage) {
        await _flutterTts!.setLanguage(currentLanguage);
        if (currentLanguage.contains('en') ||
            currentLanguage.contains('ru') ||
            currentLanguage.contains('pt') ||
            currentLanguage.contains('pl')) {
          await _flutterTts!.setSpeechRate(1);
        } else {
          await _flutterTts!.setSpeechRate(0.85);
        }
        lastLanguage = currentLanguage;
      }

      await _flutterTts!.awaitSpeakCompletion(true);

      // Switch to English and speak the language name
      if (languageName.isNotEmpty) {
        await _flutterTts!.setLanguage('en-US');
        try {
          await _flutterTts!.speak(languageName);
        } catch (e) {
          print('Error speaking the language name: $e');
        }
        // Switch back to the target language
        await _flutterTts!.setLanguage(currentLanguage);
      }

      try {
        await _flutterTts!.speak(line);
      } catch (e) {
        print('Error speaking the text: $e');
      }
    }
  }
}
