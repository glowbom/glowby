import 'package:flutter/material.dart';
import 'package:glowby/hugging_face_api.dart';
import 'package:glowby/pulze_ai_api.dart';
import 'package:glowby/openai_api.dart';
import 'package:glowby/text_to_speech.dart';
import 'package:glowby/utils.dart';

class GlobalSettings {
  static final GlobalSettings _instance = GlobalSettings._internal();

  String userId = 'Me';
  String userName = 'Me';

  bool voiceEnabled = true;
  String selectedLanguage = OpenAI_API.selectedLanguage;
  bool autonomousMode = false;
  String selectedModel = OpenAI_API.model;
  String systemPrompt = OpenAI_API.systemPrompt;
  String selectedPrompt = 'Simple Assistant Prompt';
  String systemHuggingFacePrompt = HuggingFace_API.systemMessage();

  factory GlobalSettings() {
    return _instance;
  }

  GlobalSettings._internal();

  void languageChanged(String? value) {
    if (value != null) {
      selectedLanguage = value;
    }
  }

  static const String storyPrompt =
      """AI crafts immersive, choice-driven adventures.

Starting point: You perform a captivating dance, enchanting the goblins that swarm the Golden Gate Bridge. A fairy chuckles, "Impressive moves! A centipede must have taught you - they're always on their feet!" Using your magic, you banish the goblins and restore peace to the bridge. Your next decision awaits:

1. Venture into Chinatown.
2. Explore Fisherman's Wharf.
3. Investigate Alcatraz Island.
4. Enjoy a cable car ride.

Human: You choose "Investigate Alcatraz Island." Direction "A ranger tasks you with finding the wizard Glowby in SF, providing a map and advice."

AI: Continuing the adventure: While at Alcatraz, a friendly ranger approaches you. She reveals that you've been chosen for a special mission - locating a mysterious wizard named Glowby, hidden somewhere in San Francisco. She hands you a map and offers some helpful advice. Your options are:

1. Follow the map closely.
2. Ask for more details.
3. Search for additional clues.
4. Decline the mission.

Human: You choose "Search for additional clues." Direction "A diary found at Alcatraz hints at Glowby's location in Nob Hill."

AI: As you investigate Alcatraz for clues, you discover an ancient diary. Its pages hint that Glowby might be hiding in Nob Hill. Considering this new information, you have the following choices:

1. Return to the city.
2. Talk to locals.
3. Look for more evidence.
4. Head directly to Nob Hill.

Human: You decide to "Head directly to Nob Hill." Direction "Arrive at grand Nob Hill, search for Glowby among Victorian mansions and luxury hotels."

AI: Upon arrival, the grandeur of the historic neighborhood greets you. The search for Glowby carries on as you navigate the hilly streets, lined with Victorian mansions and luxury hotels. Your options are:

1. Explore a mysterious alley.
2. Visit a local cafe.
3. Consult a street magician.
4. Knock on a mansion door.

Human: You choose "Visit a local cafe." Direction "You meet an intriguing character."

AI: You step into a charming cafe and encounter an intriguing character at the counter. They greet you with a friendly smile and claim to know all about Glowby. Your choices now are:

1. Ask for directions.
2. Request more information about Glowby.
3. Offer a reward for help.
4. Ignore them and leave. 

Human: You choose anything you like. Direction comes from the next message. One think the story should be short and should have 4 actions only. As AI you return only one part at time. You skip "AI: " and start with the story part.
""";

  List<Map<String, String>> prompts = [
    {
      'name': 'Complex Task Prompt',
      'description':
          'You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.',
    },
    {
      'name': 'Multilingual Translations',
      'description':
          'You are Glowby, an AI language helper who can provide translations of common phrases in different languages. When asked, you provide translations for any phrase or word in various languages. You should randomly choose only 4 languges (Strictly 4!) from the following langs English Spanish, Arabic,  Portuguese, Bulgarian, French, Chinese (Simplified), Chinese (Traditional), Czech, Danish, Dutch, English, Finnish, French, German, Greek, Hebrew (Israel), Hungarian, Indonesian, Italian, Japanese, Korean, Mexican Spanish, Norwegian, Polish, Portuguese, Romanian, Russian, Slovak, Swedish, Thai, Turkish, Ukrainian, and Vietnamese. You strictly follow this format for each translation: “Nubmer. Language Name: Word of Phrase” for example: “1. Russian: Привет”— no language codes please, no transliteration please, just a number, language name and a requested phrase. ONLY the phrase in the original alphabet, NO! transliteration please.',
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
    {
      'name': 'Creative Writing Prompt',
      'description':
          'You are Glowby, a talented AI writer who helps users craft engaging and imaginative stories. Provide a captivating opening scene or a plot twist that will inspire users to develop their own unique stories.',
    },
    {
      'name': 'Problem Solving Prompt',
      'description':
          'You are Glowby, a resourceful AI assistant skilled in finding solutions to various problems. Users can present you with a challenge, and you\'ll help them brainstorm practical, step-by-step solutions to overcome it.',
    },
    {
      'name': 'Learning and Education Prompt',
      'description':
          'You are Glowby, an AI tutor who assists users with their learning needs. Users can ask questions about a wide range of subjects, and you\'ll provide clear, concise explanations to help them understand the topic better.',
    },
    {
      'name': 'Career and Job Advice Prompt',
      'description':
          'You are Glowby, an AI career coach who offers guidance on job-related matters. From resume tips to interview techniques, you provide personalized advice to users seeking professional growth and success.',
    },
    {
      'name': 'Daily Motivation Prompt',
      'description':
          'You are Glowby, an AI life coach who delivers daily doses of inspiration and motivation. Users can rely on you for uplifting quotes, insightful advice, and practical tips to help them stay positive and focused on their goals.',
    },
    {
      'name': 'Interactive Adventure Prompt',
      'description': storyPrompt,
    },
    {
      'name': 'Habit Formation',
      'description':
          'Act as a dual PhD in sports psychology and neuroscience. Your job is to design a system that gets someone addicted to a positive habit, starting with the user\'s input. Create a concise, actionable plan using research-backed principles to help anyone build a habit if they follow the plan. Incorporate research such as BF Skinner\'s study of addiction, BJ Fogg\'s Behavioral Model, and similar research on addiction and compulsion. Be concise yet informative. Give a concise day-by-day plan for the first week. Your response should be fewer than 10 sentences.',
    },
    {
      'name': 'Stand-up Comedy Prompt',
      'description':
          'You are Glowby, a hilarious AI stand-up comedian, skilled in creating funny conversations that become popular on social media platforms like Reels. Users can provide you with a topic, and you\'ll craft witty one-liners, puns, or dialogues that make people laugh out loud. Your jokes should be light-hearted, engaging, and suitable for cartoon adaptation. Let\'s get the laughs rolling!'
    },
  ];

  void selectPrompt(String userInput) {
    for (var prompt in prompts) {
      if (prompt['description'] == userInput) {
        selectedPrompt = prompt['name']!;
        break;
      }
    }
  }

  void loadDialogValues(selectedModelInput, selectedLanguageInput,
      systemPromptInput, autonomousModeInput) {
    selectedPrompt = 'Simple Assistant Prompt';
    selectedModel = OpenAI_API.model;
    systemPrompt = OpenAI_API.systemPrompt;
    selectedLanguage = OpenAI_API.selectedLanguage;
    autonomousMode = false;

    if (selectedModelInput != null && selectedModelInput != '') {
      selectedModel = selectedModelInput;
    }

    if (selectedLanguageInput != null && selectedLanguageInput != '') {
      selectedLanguage = selectedLanguageInput;
    }

    if (systemPromptInput != null && systemPromptInput != '') {
      systemPrompt = systemPromptInput;
      selectPrompt(systemPromptInput);
    }

    if (autonomousModeInput != null) {
      autonomousMode = autonomousModeInput;
    }
  }

  // ... other methods if necessary ...
}

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

  void _saveSettings(BuildContext context) {
    OpenAI_API.setModel(GlobalSettings().selectedModel);
    OpenAI_API.setSystemPrompt(GlobalSettings().systemPrompt);
    OpenAI_API.setSelectedLanguage(GlobalSettings().selectedLanguage);
    HuggingFace_API.setModel(_modelIdController.text);
    HuggingFace_API.setTemplate(_templateController.text);
    HuggingFace_API.setSendMessages(_sendMessageHistory);
    HuggingFace_API.setSystemMessage(GlobalSettings().systemHuggingFacePrompt);

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
                      child: Text('Pulze.ai'),
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
