import 'dart:async';

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

  Future<void> setSpeechRate(currentLanguage) async {
    if (currentLanguage.contains('en') ||
        currentLanguage.contains('ru') ||
        currentLanguage.contains('pt') ||
        currentLanguage.contains('pl')) {
      await _flutterTts!.setSpeechRate(1);
    } else {
      await _flutterTts!.setSpeechRate(0.85);
    }
  }

  static Map<String, String> get languageCodes => _languageCodes;
  static String? lastLanguage = null;

  Future<void> speakText(String text, {String language = 'en-US'}) async {
    if (text == 'typing...') {
      return;
    }

    if (_flutterTts == null) {
      _flutterTts = FlutterTts();
    }

    bool containsLanguageCode =
        _languageCodes.keys.any((key) => text.contains(key + ':'));

    Completer<void> completer = Completer<void>();
    _flutterTts!.setCompletionHandler(() {
      completer.complete();
    });

    if (!containsLanguageCode) {
      await _flutterTts!.setLanguage(language);
      await setSpeechRate(language);
      await _flutterTts!.speak(text);
      await completer.future;
    } else {
      // Speak the initial text in the default language
      String initialText = text.split('1.')[0];
      if (initialText != '') {
        await _flutterTts!.setLanguage(language);
        await setSpeechRate(language);
        await _flutterTts!.speak(initialText);
        await completer.future;
        completer = Completer<void>();
      }

      List<String> lines = text.split('\n');
      for (String line in lines) {
        String currentLanguage = language;
        for (final entry in _languageCodes.entries) {
          if (line.contains(entry.key + ':')) {
            // Speak the part before the colon with the default language.
            String beforeColon = line.split(entry.key + ':')[0];
            if (beforeColon != '') {
              await _flutterTts!.setLanguage(currentLanguage);
              await setSpeechRate(currentLanguage);
              await _flutterTts!.speak(beforeColon + ' ' + entry.key);
              await completer.future;
              completer = Completer<void>();
            }

            // Speak the part after the colon with the appropriate language.
            String afterColon = line.split(entry.key + ':')[1];
            currentLanguage = entry.value;
            await _flutterTts!.setLanguage(currentLanguage);
            await setSpeechRate(currentLanguage);
            await _flutterTts!.speak(afterColon);
            await completer.future;
            completer = Completer<void>();
            break;
          }
        }
      }
    }
  }
}
