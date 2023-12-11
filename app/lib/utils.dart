import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;
import 'dart:html' as html;

import 'package:url_launcher/url_launcher_string.dart';

class Utils {
  static Future<void> launchURL(String url) async {
    if (await canLaunchUrlString(url)) {
      await launchUrlString(url);
    } else {
      throw Exception('Could not launch $url');
    }
  }

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
        throw Exception('Failed to download image: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to save image: ${e.toString()}');
    }
  }

  static List<String> imageReadyMessages = [
    'Your image is ready! Make sure you allow pop-ups.',
    'Image generated! Remember to enable pop-ups.',
    'Voila! Your image is done. Don\'t forget to allow pop-ups.',
    'Success! Your image has been created. Ensure pop-ups are allowed.',
    'Image complete! Just a reminder to permit pop-ups.',
    'Your masterpiece is finished! Please enable pop-ups.',
    'Your visual creation is ready! Check that you\'ve allowed pop-ups.',
  ];

  static String getRandomImageReadyMessage() {
    final random = Random();
    int index = random.nextInt(imageReadyMessages.length);
    return imageReadyMessages[index];
  }

  static List<String> imageGenerationFunnyMessages = [
    'Drawing your image... Searching for my digital paintbrush...',
    'Drawing your image... Brewing a colorful potion...',
    'Drawing your image... Summoning artistic inspiration...',
    'Drawing your image... Painting with pixels...',
    'Drawing your image... Conjuring a visual masterpiece...',
    'Drawing your image... Diving into the canvas of imagination...',
    'Drawing your image... Sketching with code...',
    'Drawing your image... Weaving a tapestry of pixels...',
    'Drawing your image... Navigating the art labyrinth...',
    'Drawing your image... Decoding visual enigmas...',
    'Drawing your image... Assembling a digital gallery...',
    'Drawing your image... Cooking up a visual feast...',
    'Drawing your image... Unraveling the threads of creativity...',
    'Drawing your image... Tapping into the visual matrix...',
    'Drawing your image... Beaming up new designs...',
    'Drawing your image... Hitchhiking through the artverse...',
    'Drawing your image... Dusting off ancient palettes...',
    'Drawing your image... Crafting an artistic potion...',
    'Drawing your image... Igniting the creative spark...',
    'Drawing your image... Fishing for inspiration...',
    'Drawing your image... Sifting through the sands of design...',
    'Drawing your image... Gathering visual gems...',
    'Drawing your image... Riding the wave of imagination...',
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
