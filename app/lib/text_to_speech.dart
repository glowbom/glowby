import 'package:flutter_tts/flutter_tts.dart';

class TextToSpeech {
  FlutterTts? _flutterTts;

  final Map<String, String> _languageCodes = {
    'Italian': 'it-IT',
    'German': 'de-DE',
    'Portuguese': 'pt-PT',
    'Dutch': 'nl-NL',
    'Russian': 'ru-RU',
    'American Spanish': 'es-US',
    'Mexican Spanish': 'es-MX',
    'Canadian French': 'fr-CA',
    'French': 'fr-FR',
    'Spanish': 'es-ES',
    'American English': 'en-US',
    'British English': 'en-GB',
    'Australian English': 'en-AU',
    'English': 'en-US',
  };

  Future<void> speakText(String text, {String language = 'English'}) async {
    if (text == 'typing...') {
      return;
    }

    if (_flutterTts == null) {
      _flutterTts = FlutterTts();
    }

    String? languageCode = _languageCodes[language];
    if (languageCode != null) {
      await _flutterTts!.setLanguage(languageCode);
    } else {
      print('Invalid language provided. Falling back to default language.');
      await _flutterTts!.setLanguage(_languageCodes['English']!);
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
