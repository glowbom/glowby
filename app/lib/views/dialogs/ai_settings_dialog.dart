import 'package:flutter/material.dart';
import 'package:glowby/services/network.dart';
import 'package:glowby/views/screens/global_settings.dart';
import 'package:glowby/services/pulze_ai_api.dart';
import 'package:glowby/utils/text_to_speech.dart';
import 'package:glowby/utils/utils.dart';

class AiSettingsDialog extends StatefulWidget {
  final Function(bool) onVoiceEnabledChanged;

  const AiSettingsDialog({super.key, required this.onVoiceEnabledChanged});

  @override
  AiSettingsDialogState createState() => AiSettingsDialogState();
}

class AiSettingsDialogState extends State<AiSettingsDialog> {
  bool _isPulzeSelected = false;

  final TextEditingController _systemPromptController = TextEditingController();
  final TextEditingController _pulzeModelIdController = TextEditingController();

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
    GlobalSettings().selectedModel = 'pulzeai';
    _systemPromptController.text = GlobalSettings().systemPrompt;
    //_isGPT4Selected =
    //    _selectedModel == 'gpt-4' || _selectedModel == 'gpt-4-1106-preview';
    _isPulzeSelected = GlobalSettings().selectedModel == 'pulzeai';
    _pulzeModelIdController.text = PulzeAiApi.model();
  }

  void _saveOpenAISettings() {
    Network.setModel(GlobalSettings().selectedModel);
    Network.setSystemPrompt(GlobalSettings().systemPrompt);
    Network.setSelectedLanguage(GlobalSettings().selectedLanguage);
  }

  void _saveHuggingFaceSettings() {
    PulzeAiApi.setModel(_pulzeModelIdController.text);
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

  Future<List<String>> _getActiveModels() async {
    List<dynamic> models = await PulzeAiApi.getActiveModels();
    List<String> namespaces =
        models.map((e) => e['namespace'] as String).toList();

    // Remove 'pulze' if it exists in the list
    namespaces.remove('pulze');
    // Insert 'pulze' at the beginning of the list
    namespaces.insert(0, 'pulze');

    return namespaces;
  }

  Widget _buildModelIdDropdown() {
    return FutureBuilder<List<String>>(
      future: _getActiveModels(),
      builder: (BuildContext context, AsyncSnapshot<List<String>> snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const CircularProgressIndicator();
        } else if (snapshot.hasError) {
          return Text('Error: ${snapshot.error}');
        } else {
          return DropdownButton<String>(
            value: PulzeAiApi.model(),
            items: snapshot.data!.map((String value) {
              return DropdownMenuItem<String>(
                value: value,
                child: Text(value),
              );
            }).toList(),
            onChanged: (String? newValue) {
              setState(() {
                _pulzeModelIdController.text = newValue!;
                PulzeAiApi.setModel(newValue);
              });
            },
          );
        }
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('AI Settings'),
      content: SizedBox(
        width: 340, // Set the max width of the AlertDialog
        child: SingleChildScrollView(
          child: ListBody(
            children: <Widget>[
              if (_isPulzeSelected)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    const SizedBox(height: 10),
                    const Text('Model ID:'),
                    const SizedBox(height: 6),
                    /*InkWell(
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
                    ),*/
                    _buildModelIdDropdown(),
                  ],
                ),
              const SizedBox(height: 10),
              const Text('System Prompt:'),
              DropdownButton<String>(
                value: GlobalSettings().selectedPrompt,
                items: buildPromptDropdownItems(),
                onChanged: (value) {
                  setState(() {
                    _promptChanged(value);
                  });
                },
              ),
              _buildAutonomousModeCheckbox(),
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
