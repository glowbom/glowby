import 'package:flutter/material.dart';
import 'package:glowby/global_settings.dart';
import 'package:glowby/hugging_face_api.dart';
import 'package:glowby/pulze_ai_api.dart';
import 'package:glowby/openai_api.dart';
import 'package:glowby/text_to_speech.dart';
import 'package:glowby/utils.dart';

class AiSettingsDialog extends StatefulWidget {
  final Function(bool) onVoiceEnabledChanged;

  AiSettingsDialog({required this.onVoiceEnabledChanged});

  @override
  _AiSettingsDialogState createState() => _AiSettingsDialogState();
}

class _AiSettingsDialogState extends State<AiSettingsDialog> {
  bool _isHuggingFaceSelected = false;
  bool _sendMessageHistory = false;

  final TextEditingController _systemPromptController = TextEditingController();
  final TextEditingController _systemPromptHuggingFaceController =
      TextEditingController();
  final TextEditingController _modelIdController = TextEditingController();
  final TextEditingController _templateController = TextEditingController();

  Widget _buildAutonomousModeCheckbox() {
    if (GlobalSettings().selectedPrompt == 'Complex Task Prompt') {
      return CheckboxListTile(
        title: Text('Autonomous Mode (Experimental)'),
        value: GlobalSettings().autonomousMode,
        onChanged: (bool? value) {
          setState(() {
            GlobalSettings().autonomousMode = value!;
          });
        },
      );
    }
    return SizedBox.shrink();
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

  List<DropdownMenuItem<String>> buildPromptDropdownItems() {
    return GlobalSettings()
        .prompts
        .map((prompt) => DropdownMenuItem<String>(
              value: prompt['name'],
              child: Text(prompt['name']!),
            ))
        .toList();
  }

  void _promptChanged(String? value) {
    if (value != null) {
      GlobalSettings().selectedPrompt = value;
      Map<String, dynamic> selectedPromptMap = GlobalSettings()
          .prompts
          .firstWhere((prompt) => prompt['name'] == value,
              orElse: () =>
                  {'name': value, 'description': ''} // Provide a default map
              );

      GlobalSettings().systemPrompt = selectedPromptMap['description'] ?? '';
      _systemPromptController.text = GlobalSettings().systemPrompt;
      GlobalSettings().autonomousMode = false;
    }
  }

  @override
  void initState() {
    super.initState();
    GlobalSettings().selectedModel = OpenAI_API.model;
    _systemPromptController.text = GlobalSettings().systemPrompt;
    //_isGPT4Selected =
    //    _selectedModel == 'gpt-4' || _selectedModel == 'gpt-4-1106-preview';
    _isHuggingFaceSelected = GlobalSettings().selectedModel == 'huggingface';
    _modelIdController.text = HuggingFace_API.model();
    _templateController.text = HuggingFace_API.template();
    GlobalSettings().systemHuggingFacePrompt = HuggingFace_API.systemMessage();
    _sendMessageHistory = HuggingFace_API.sendMessages();
    _systemPromptHuggingFaceController.text =
        GlobalSettings().systemHuggingFacePrompt;
  }

  void _saveOpenAISettings() {
    OpenAI_API.setModel(GlobalSettings().selectedModel);
    OpenAI_API.setSystemPrompt(GlobalSettings().systemPrompt);
    OpenAI_API.setSelectedLanguage(GlobalSettings().selectedLanguage);
  }

  void _saveHuggingFaceSettings() {
    HuggingFace_API.setModel(_modelIdController.text);
    HuggingFace_API.setTemplate(_templateController.text);
    HuggingFace_API.setSendMessages(_sendMessageHistory);
    HuggingFace_API.setSystemMessage(GlobalSettings().systemHuggingFacePrompt);
  }

  void _saveSettings(BuildContext context) {
    _saveOpenAISettings();
    _saveHuggingFaceSettings();

    // Save the system prompt to use with API calls
    Navigator.pop(context); // Hide the dialog

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('AI Settings saved successfully!')),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (HuggingFace_API.oat() == '' &&
        GlobalSettings().selectedModel == 'huggingface') {
      OpenAI_API.setModel(GlobalSettings().selectedModel);
      setState(() {
        _isHuggingFaceSelected = false;
        //_isGPT4Selected = false;
        GlobalSettings().selectedModel = 'gpt-3.5-turbo';
      });
    }
    return AlertDialog(
      title: Text('AI Settings'),
      content: Container(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              Text('Choose AI Model:'),
              DropdownButton<String>(
                value: GlobalSettings().selectedModel,
                items: [
                  DropdownMenuItem<String>(
                    value: 'gpt-3.5-turbo',
                    child: Text('GPT-3.5 (Recommended)'),
                  ),
                  DropdownMenuItem<String>(
                    value: 'gpt-4',
                    child: Text('GPT-4 (Advanced)'),
                  ),
                  DropdownMenuItem<String>(
                    value: 'gpt-4-1106-preview',
                    child: Text('GPT-4 Turbo (Preview)'),
                  ),
                  if (HuggingFace_API.oat() != '')
                    DropdownMenuItem<String>(
                      value: 'huggingface',
                      child: Text('Hugging Face (Experimental)'),
                    ),
                  if (PulzeAI_API.oat() != '')
                    DropdownMenuItem<String>(
                      value: 'pulzeai',
                      child: Text('Pulze'),
                    ),
                  /*DropdownMenuItem<String>(
                    value: 'gpt-4-32k',
                    child: Text('GPT-4-32k (Advanced, Limited Beta)'),
                  ),*/
                  // uncomment the following lines to enable an extended 32,000 token context-length model gpt-4-32k
                ],
                onChanged: (value) {
                  setState(() {
                    GlobalSettings().selectedModel = value!;
                    //_isGPT4Selected =
                    //    value == 'gpt-4' || value == 'gpt-4-1106-preview';
                    _isHuggingFaceSelected = value == 'huggingface';
                  });
                },
              ),
              SizedBox(height: 10),
              if (_isHuggingFaceSelected)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    SizedBox(height: 10),
                    Text('Hugging Face Model ID:'),
                    SizedBox(height: 6),
                    InkWell(
                      child: Text(
                        '→ Browse available models',
                        style: TextStyle(color: Colors.blue),
                      ),
                      onTap: () => Utils.launchURL(
                          'https://huggingface.co/models?pipeline_tag=text2text-generation&sort=downloads'),
                    ),
                    TextField(
                      controller:
                          _modelIdController, // Use TextEditingController to retrieve user input
                      decoration: InputDecoration(
                        labelText: 'Model ID',
                      ),
                      onChanged: (value) {
                        // Update your modelId variable here
                      },
                    ),
                    SizedBox(height: 10),
                    Text('Response Format'),
                    TextField(
                      controller:
                          _templateController, // Use TextEditingController to retrieve user input
                      maxLines: 5,
                      decoration: InputDecoration(
                        labelText: 'Template (*** is the response)',
                      ),
                      onChanged: (value) {
                        // Update your template variable here
                      },
                    ),
                  ],
                ),
              SizedBox(height: 10),
              if (!_isHuggingFaceSelected) Text('System Prompt:'),
              if (!_isHuggingFaceSelected)
                if (!_isHuggingFaceSelected)
                  DropdownButton<String>(
                    value: GlobalSettings().selectedPrompt,
                    items: buildPromptDropdownItems(),
                    onChanged: (value) {
                      setState(() {
                        _promptChanged(value);
                      });
                    },
                  ),
              if (!_isHuggingFaceSelected) _buildAutonomousModeCheckbox(),
              if (!_isHuggingFaceSelected)
                TextField(
                  controller: _systemPromptController,
                  maxLines: 3,
                  decoration: InputDecoration(
                    labelText: 'Enter system prompt',
                  ),
                  onChanged: (value) {
                    GlobalSettings().systemPrompt = value;
                  },
                ),
              /*if (_isHuggingFaceSelected) Text('System Message:'),
              if (_isHuggingFaceSelected)
                TextField(
                  controller: _systemPromptHuggingFaceController,
                  maxLines: 3,
                  decoration: InputDecoration(
                    labelText: 'Enter system message',
                  ),
                  onChanged: (value) {
                    _systemHuggingFacePrompt = value;
                  },
                ),
              if (_isHuggingFaceSelected)
                CheckboxListTile(
                  title: Text('Send message history'),
                  value: _sendMessageHistory,
                  onChanged: (bool? value) {
                    setState(() {
                      _sendMessageHistory = value!;
                    });
                  },
                ),
              if (_isHuggingFaceSelected) Divider(),
              if (_isHuggingFaceSelected) Text('Voice Settings:'),
              if (_isHuggingFaceSelected) SizedBox(height: 10),*/
              CheckboxListTile(
                title: Text('Enable voice'),
                value: GlobalSettings().voiceEnabled,
                onChanged: (bool? value) {
                  setState(() {
                    GlobalSettings().voiceEnabled = value!;
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
                    value: GlobalSettings().selectedLanguage,
                    items: buildLanguageDropdownItems(),
                    onChanged: (value) {
                      setState(() {
                        GlobalSettings().languageChanged(value);
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
          child: Text(
            'Save Settings',
            style: TextStyle(color: Colors.white),
          ),
          onPressed: () => _saveSettings(context),
        ),
      ],
    );
  }
}
