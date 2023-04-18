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

    if (language.isNotEmpty && language != lastLanguage) {
      await _flutterTts!.setLanguage(language);
      lastLanguage = language;
    }

    for (final entry in _languageCodes.entries) {
      if (text.startsWith('${entry.key}: ')) {
        await _flutterTts!.setLanguage(entry.value);
        text = text.replaceAll('${entry.key}: ', '');
        break;
      }
    }

    await _flutterTts!.awaitSpeakCompletion(true);
    try {
      await _flutterTts!.speak(text);
    } catch (e) {
      print('Error speaking the text: $e');
    }
  }
}
