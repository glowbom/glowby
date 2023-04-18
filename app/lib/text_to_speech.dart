import 'package:flutter_tts/flutter_tts.dart';

class TextToSpeech {
  FlutterTts? _flutterTts;

  static final Map<String, String> _languageCodes = {
    'American English': 'en-US',
    'American Spanish': 'es-US',
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
    'French': 'fr-FR',
    'German': 'de-DE',
    'Italian': 'it-IT',
    'Japanese': 'ja-JP',
    'Korean': 'ko-KR',
    'Mexican Spanish': 'es-MX',
    'Norwegian': 'nb-NO',
    'Polish': 'pl-PL',
    'Portuguese': 'pt-PT',
    'Russian': 'ru-RU',
    'Spanish': 'es-ES',
    'Swedish': 'sv-SE',
    'Ukrainian': 'uk-UA',
    'Finnish': 'fi-FI',
    'Arabic (Saudi Arabia)': 'ar-SA',
    'Greek': 'el-GR',
    'Hebrew (Israel)': 'he-IL',
    'Hungarian': 'hu-HU',
    'Indonesian': 'id-ID',
    'Romanian': 'ro-RO',
    'Slovak': 'sk-SK',
    'Thai': 'th-TH',
    'Turkish': 'tr-TR',
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
