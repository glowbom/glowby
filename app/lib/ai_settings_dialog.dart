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

  static String _selectedLanguage = OpenAI_API.selectedLanguage;

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

  static String _selectedPrompt = 'Complex Task Prompt';
  List<Map<String, String>> _prompts = [
    {
      'name': 'Complex Task Prompt',
      'description':
          'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.',
    },
    {
      'name': 'Brainstorming Prompt',
      'description':
          'Generate ideas with Glowby! As a super helpful, nice, and humorous AI assistant, Glowby is ready to provide you with a concise plan and assist in executing it. With Glowby by your side, you\'ll never feel stuck again. Let\'s get brainstorming!',
    },
    {
      'name': 'Simple Assistant Prompt',
      'description':
          'You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.',
    },
  ];

  List<DropdownMenuItem<String>> buildPromptDropdownItems() {
    return _prompts
        .map((prompt) => DropdownMenuItem<String>(
              value: prompt['name'],
              child: Text(prompt['name']!),
            ))
        .toList();
  }

  void _promptChanged(String? value) {
    if (value != null) {
      _selectedPrompt = value;
      _systemPrompt = _prompts.firstWhere(
          (prompt) => prompt['name'] == _selectedPrompt)['description']!;
      _systemPromptController.text = _systemPrompt;
    }
  }

  @override
  void initState() {
    super.initState();
    _systemPromptController.text = _systemPrompt;
  }

  void _saveSettings(BuildContext context) {
    OpenAI_API.setModel(_selectedModel);
    OpenAI_API.setSystemPrompt(_systemPrompt);
    OpenAI_API.setSelectedLanguage(_selectedLanguage);

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
              DropdownButton<String>(
                value: _selectedPrompt,
                items: buildPromptDropdownItems(),
                onChanged: (value) {
                  setState(() {
                    _promptChanged(value);
                  });
                },
              ),
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
              Padding(
                padding: const EdgeInsets.only(left: 12, right: 20),
                child: Container(
                  width: 220, // Width adjusted to match expanding triangle
                  child: DropdownButton<String>(
                    isExpanded: true,
                    value: _selectedLanguage,
                    items: buildLanguageDropdownItems(),
                    onChanged: (value) {
                      setState(() {
                        _languageChanged(value);
                      });
                    },
                  ),
                ),
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
