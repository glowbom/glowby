import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;
import 'dart:html' as html;

class Utils {
  // List of image command patterns
  static List<String> imageCommandPatterns = [
    r'\b(draw|paint|generate|create|show) (me )?(a |an )?(pic|picture|image|illustration|drawing)\b',
    r'\b(draw|paint|generate|create|show) this (for me)?\b',
    r'\b(can you )?(please )?(draw|paint|generate|create|show) (me )?(a |an )?(pic|picture|image|illustration|drawing)\b',
    r'\b(i want to see|show me) (a |an )?(pic|picture|image|illustration|drawing) of\b',
  ];

  // Function to test if any command patterns match the user input
  static bool isImageGenerationCommand(String input) {
    return imageCommandPatterns.any((pattern) {
      RegExp regex = RegExp(pattern, caseSensitive: false);
      return regex.hasMatch(input);
    });
  }

  static String? getMatchingPattern(String input) {
    for (var pattern in imageCommandPatterns) {
      RegExp regex = RegExp(pattern, caseSensitive: false);
      if (regex.hasMatch(input)) {
        return pattern;
      }
    }
    return null;
  }

  static void downloadImage(String url, String description) {
    final windowFeatures =
        'menubar=no,toolbar=no,status=no,resizable=yes,scrollbars=yes,width=600,height=400';
    html.window.open(url, 'glowby-image-${description}', windowFeatures);
  }

  static Future<String> getImageDataFromUrl(String url) async {
    try {
      final response = await http.get(Uri.parse(url));
      if (response.statusCode == 200) {
        final imageData = response.bodyBytes;
        final base64Image = base64Encode(imageData);
        return base64Image;
      } else {
        throw 'Failed to download image: ${response.statusCode}';
      }
    } catch (e) {
      throw 'Failed to save image: ${e.toString()}';
    }
  }

  static List<String> imageGenerationFunnyMessages = [
    'Searching for my digital paintbrush...',
    'Brewing a colorful potion...',
    'Summoning artistic inspiration...',
    'Painting with pixels...',
    'Conjuring a visual masterpiece...',
    'Diving into the canvas of imagination...',
    'Sketching with code...',
    'Weaving a tapestry of pixels...',
    'Navigating the art labyrinth...',
    'Decoding visual enigmas...',
    'Assembling a digital gallery...',
    'Cooking up a visual feast...',
    'Unraveling the threads of creativity...',
    'Tapping into the visual matrix...',
    'Beaming up new designs...',
    'Hitchhiking through the artverse...',
    'Dusting off ancient palettes...',
    'Crafting an artistic potion...',
    'Igniting the creative spark...',
    'Fishing for inspiration...',
    'Sifting through the sands of design...',
    'Gathering visual gems...',
    'Riding the wave of imagination...',
  ];

  static String getRandomImageGenerationFunnyMessage() {
    final random = Random();
    int index = random.nextInt(imageGenerationFunnyMessages.length);
    return imageGenerationFunnyMessages[index];
  }

  static List<String> loadingMessages = [
    'Talking to collective wisdom...',
    'Summoning creative spirits...',
    'Traveling through the storyverse...',
    'Weaving a tale of wonder...',
    'Fetching new story elements...',
    'Conjuring the next adventure...',
    'Spinning the wheel of stories...',
    'Navigating the plot labyrinth...',
    'Polishing plot twists...',
    'Discovering new realms...',
    'Replenishing the inkwell...',
    'Crafting a narrative potion...',
    'Unraveling the yarn of tales...',
    'Tapping into the story matrix...',
    'Beaming up new ideas...',
    'Hitchhiking through fiction...',
    'Dusting off ancient tomes...',
    'Decoding narrative enigmas...',
    'Assembling a cast of characters...',
    'Cooking up a literary feast...',
    'Diving into the imagination ocean...',
    'Venturing into narrative territory...',
    'Harvesting ideas from the idea tree...',
    'Rolling out the story carpet...',
    'Unlocking the tale treasure chest...',
    'Painting with words...',
    'Whispering to the muses...',
    'Peering through the story telescope...',
    'Igniting the creative spark...',
    'Fishing for inspiration...',
    'Sifting through story sands...',
    'Gathering narrative gems...',
    'Riding the wave of imagination...',
  ];

  static String getRandomMessage() {
    final random = Random();
    int index = random.nextInt(loadingMessages.length);
    return loadingMessages[index];
  }

  static List<String> loadingMessagesForCode = [
    'Compiling the code of creation...',
    'Brewing a batch of algorithms...',
    'Assembling the building blocks...',
    'Constructing digital blueprints...',
    'Wiring up the circuits of logic...',
    'Unraveling the threads of code...',
    'Mining the data mines...',
    'Sculpting the logic landscape...',
    'Connecting the code constellations...',
    'Weaving the web of functions...',
    'Chasing the elusive bugs...',
    'Deciphering the secrets of syntax...',
    'Calibrating the code compass...',
    'Setting sail on the sea of scripts...',
    'Delving into the depths of data...',
    'Energizing the flow of logic...',
    'Exploring the code caves...',
    'Tapping into the digital wellspring...',
    'Navigating the river of routines...',
    'Mapping the code cosmos...',
    'Solving the algorithmic riddles...',
    'Stitching together the fabric of code...',
    'Tuning the code symphony...',
    'Traversing the code jungle...',
    'Unlocking the digital vault...',
    'Harnessing the power of variables...',
    'Journeying through the function forest...',
    'Carving the code canyon...',
    'Gathering the pearls of programming...',
    'Pioneering the code frontier...',
    'Unleashing the power of progress...',
  ];

  static String getRandomMessageForCode() {
    final random = Random();
    int index = random.nextInt(loadingMessagesForCode.length);
    return loadingMessagesForCode[index];
  }
}
