import 'package:flutter/material.dart';
import 'package:glowby/views/screens/global_settings.dart';
import 'package:glowby/services/hugging_face_api.dart';
import 'package:glowby/services/pulze_ai_api.dart';
import 'package:glowby/services/openai_api.dart';
import 'package:glowby/utils/text_to_speech.dart';
import 'package:glowby/utils/utils.dart';

class AiSettingsDialog extends StatefulWidget {
  final Function(bool) onVoiceEnabledChanged;

  const AiSettingsDialog({super.key, required this.onVoiceEnabledChanged});

  @override
  AiSettingsDialogState createState() => AiSettingsDialogState();
}

class AiSettingsDialogState extends State<AiSettingsDialog> {
  bool _isHuggingFaceSelected = false;
  bool _isPulzeSelected = false;
  bool _sendMessageHistory = false;

  final TextEditingController _systemPromptController = TextEditingController();
  final TextEditingController _systemPromptHuggingFaceController =
      TextEditingController();
  final TextEditingController _modelIdController = TextEditingController();
  final TextEditingController _pulzeModelIdController = TextEditingController();
  final TextEditingController _templateController = TextEditingController();

  Widget _buildAutonomousModeCheckbox() {
    if (GlobalSettings().selectedPrompt == 'Complex Task Prompt') {
      return CheckboxListTile(
        title: const Text('Autonomous Mode (Experimental)'),
        value: GlobalSettings().autonomousMode,
        onChanged: (bool? value) {
          setState(() {
            GlobalSettings().autonomousMode = value!;
          });
        },
      );
    }
    return const SizedBox.shrink();
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
    GlobalSettings().selectedModel = OpenAiApi.model;
    _systemPromptController.text = GlobalSettings().systemPrompt;
    _isHuggingFaceSelected = GlobalSettings().selectedModel == 'huggingface';
    _isPulzeSelected = GlobalSettings().selectedModel == 'pulzeai';
    _modelIdController.text = HuggingFaceApi.model();
    _pulzeModelIdController.text = PulzeAiApi.model();
    _templateController.text = HuggingFaceApi.template();
    GlobalSettings().systemHuggingFacePrompt = HuggingFaceApi.systemMessage();
    _sendMessageHistory = HuggingFaceApi.sendMessages();
    _systemPromptHuggingFaceController.text =
        GlobalSettings().systemHuggingFacePrompt;
  }

  void _saveOpenAISettings() {
    OpenAiApi.setModel(GlobalSettings().selectedModel);
    OpenAiApi.setSystemPrompt(GlobalSettings().systemPrompt);
    OpenAiApi.setSelectedLanguage(GlobalSettings().selectedLanguage);
  }

  void _saveHuggingFaceSettings() {
    PulzeAiApi.setModel(_pulzeModelIdController.text);
    HuggingFaceApi.setModel(_modelIdController.text);
    HuggingFaceApi.setTemplate(_templateController.text);
    HuggingFaceApi.setSendMessages(_sendMessageHistory);
    HuggingFaceApi.setSystemMessage(GlobalSettings().systemHuggingFacePrompt);
  }

  void _saveSettings(BuildContext context) {
    _saveOpenAISettings();
    _saveHuggingFaceSettings();

    // Save the system prompt to use with API calls
    Navigator.pop(context); // Hide the dialog

    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('AI Settings saved successfully!')),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (HuggingFaceApi.oat() == '' &&
        GlobalSettings().selectedModel == 'huggingface') {
      OpenAiApi.setModel(GlobalSettings().selectedModel);
      setState(() {
        _isHuggingFaceSelected = false;
        _isPulzeSelected = false;
        //_isGPT4Selected = false;
        GlobalSettings().selectedModel = 'gpt-4o';
      });
    }
    return AlertDialog(
      title: const Text('AI Settings'),
      content: SizedBox(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              const Text('Choose AI Model or Provider:'),
              DropdownButton<String>(
                value: GlobalSettings().selectedModel,
                items: [
                  const DropdownMenuItem<String>(
                    value: 'gpt-4o',
                    child: Text('GPT-4o (Recommended)'),
                  ),
                  const DropdownMenuItem<String>(
                    value: 'gpt-4',
                    child: Text('GPT-4'),
                  ),
                  const DropdownMenuItem<String>(
                    value: 'gpt-3.5-turbo',
                    child: Text('GPT-3.5'),
                  ),
                  if (HuggingFaceApi.oat() != '')
                    const DropdownMenuItem<String>(
                      value: 'huggingface',
                      child: Text('Hugging Face (Experimental)'),
                    ),
                  if (PulzeAiApi.oat() != '')
                    const DropdownMenuItem<String>(
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
                    _isHuggingFaceSelected = value == 'huggingface';
                    _isPulzeSelected = value == 'pulzeai';
                  });
                },
              ),
              const SizedBox(height: 10),
              if (_isPulzeSelected)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    const SizedBox(height: 10),
                    const Text('Model ID:'),
                    const SizedBox(height: 6),
                    InkWell(
                      child: const Text(
                        '→ Browse available models',
                        style: TextStyle(color: Colors.blue),
                      ),
                      onTap: () =>
                          Utils.launchURL('https://platform.pulze.ai/models'),
                    ),
                    TextField(
                      controller:
                          _pulzeModelIdController, // Use TextEditingController to retrieve user input
                      decoration: const InputDecoration(
                        labelText: 'Model ID',
                      ),
                      onChanged: (value) {
                        // Update your modelId variable here
                      },
                    ),
                  ],
                ),
              if (_isHuggingFaceSelected)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    const SizedBox(height: 10),
                    const Text('Hugging Face Model ID:'),
                    const SizedBox(height: 6),
                    InkWell(
                      child: const Text(
                        '→ Browse available models',
                        style: TextStyle(color: Colors.blue),
                      ),
                      onTap: () => Utils.launchURL(
                          'https://huggingface.co/models?pipeline_tag=text2text-generation&sort=downloads'),
                    ),
                    TextField(
                      controller:
                          _modelIdController, // Use TextEditingController to retrieve user input
                      decoration: const InputDecoration(
                        labelText: 'Model ID',
                      ),
                      onChanged: (value) {
                        // Update your modelId variable here
                      },
                    ),
                    const SizedBox(height: 10),
                    const Text('Response Format'),
                    TextField(
                      controller:
                          _templateController, // Use TextEditingController to retrieve user input
                      maxLines: 5,
                      decoration: const InputDecoration(
                        labelText: 'Template (*** is the response)',
                      ),
                      onChanged: (value) {
                        // Update your template variable here
                      },
                    ),
                  ],
                ),
              const SizedBox(height: 10),
              if (!_isHuggingFaceSelected) const Text('System Prompt:'),
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
                  decoration: const InputDecoration(
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
                title: const Text('Enable voice'),
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
                child: SizedBox(
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
          child: const Text('Cancel'),
          onPressed: () {
            Navigator.pop(context);
          },
        ),
        ElevatedButton(
          child: const Text(
            'Save Settings',
            style: TextStyle(color: Colors.white),
          ),
          onPressed: () => _saveSettings(context),
        ),
      ],
    );
  }
}
