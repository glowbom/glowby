import 'package:flutter/material.dart';
import 'package:web/openai_api.dart';
import 'package:web/text_to_speech.dart';

import 'chat_screen.dart';

class AiSettingsDialog extends StatefulWidget {
  final Function(bool) onVoiceEnabledChanged;

  AiSettingsDialog({required this.onVoiceEnabledChanged});

  static String get selectedLanguage =>
      _AiSettingsDialogState._selectedLanguage;

  @override
  _AiSettingsDialogState createState() => _AiSettingsDialogState();
}

class _AiSettingsDialogState extends State<AiSettingsDialog> {
  String _selectedModel = OpenAI_API.model;
  String _systemPrompt = OpenAI_API.systemPrompt;
  final TextEditingController _systemPromptController = TextEditingController();

  static String _selectedLanguage = 'en-US';

  static void _languageChanged(String? value) {
    if (value != null) {
      _selectedLanguage = value;
    }
  }

  List<DropdownMenuItem<String>> buildLanguageDropdownItems() {
    Set<String> uniqueLanguageCodes =
        Set<String>.from(TextToSpeech.languageCodes.values);
    return uniqueLanguageCodes
        .map((code) => DropdownMenuItem<String>(
              value: code,
              child: Text(TextToSpeech.languageCodes.entries
                  .firstWhere((entry) => entry.value == code)
                  .key),
            ))
        .toList();
  }

  @override
  void initState() {
    super.initState();
    _systemPromptController.text = _systemPrompt;
  }

  void _saveSettings(BuildContext context) {
    OpenAI_API.setModel(_selectedModel);
    OpenAI_API.setSystemPrompt(_systemPrompt);

    // Save the system prompt to use with API calls
    Navigator.pop(context); // Hide the dialog

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('AI Settings saved successfully!')),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text('AI Settings'),
      content: Container(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              Text('Choose AI Model:'),
              DropdownButton<String>(
                value: _selectedModel,
                items: [
                  DropdownMenuItem<String>(
                    value: 'gpt-3.5-turbo',
                    child: Text('GPT-3.5 Turbo (Recommended)'),
                  ),
                  DropdownMenuItem<String>(
                    value: 'gpt-4',
                    child: Text('GPT-4 (Advanced, Limited Beta)'),
                  ),
                ],
                onChanged: (value) {
                  setState(() {
                    _selectedModel = value!;
                  });
                },
              ),
              SizedBox(height: 10),
              Text('System Prompt:'),
              TextField(
                controller: _systemPromptController,
                maxLines: 3,
                decoration: InputDecoration(
                  labelText: 'Enter system prompt',
                ),
                onChanged: (value) {
                  _systemPrompt = value;
                },
              ),
              CheckboxListTile(
                title: Text('Enable voice'),
                value: ChatScreenState.voiceEnabled,
                onChanged: (bool? value) {
                  setState(() {
                    ChatScreenState.voiceEnabled = value!;
                  });
                  widget.onVoiceEnabledChanged(value!);
                },
              ),
              SizedBox(height: 10),
              Text('Choose Language:'),
              DropdownButton<String>(
                value: _selectedLanguage,
                items: buildLanguageDropdownItems(),
                onChanged: _languageChanged,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          child: Text('Cancel'),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
        ElevatedButton(
          child: Text('Save Settings'),
          onPressed: () => _saveSettings(context),
        ),
      ],
    );
  }
}
