# Glowby Basic - Customizable Voice-Enabled AI Assistant

Glowby Basic is a powerful voice-based AI assistant that can help users with various tasks. Easily customizable, trainable, and deployable anywhere, Glowby Basic is designed to adapt to your specific needs. Built using Flutter, Glowby Basic provides a seamless web app experience with an intuitive voice interface.

## Live Demo

Experience Glowby Basic in action with our live demo hosted on GitHub Pages [here](https://glowbom.github.io/glowby-basic/).


![GitHub Repo stars](https://img.shields.io/github/stars/glowbom/glowby?style=social)
[![Twitter Follow](https://img.shields.io/twitter/follow/GlowbomCorp?style=social)](https://twitter.com/GlowbomCorp)
[![Discord Follow](https://dcbadge.vercel.app/api/server/zqcsurUN?style=flat)](https://discord.gg/zqcsurUN)

### See It in Action

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232182586-30984d36-4641-41da-9e1e-c23c27716e3d.gif)


## Overview

This project offers an easy way for creating customizable AI assistants like [Glowby](https://www.youtube.com/watch?v=iFECpMXmKOg), a witty AI agent that assists users in building apps on [Glowbom.com](https://www.glowbom.com). By open-sourcing the Flutter-based chat component, we aim to foster a community-driven ecosystem to build diverse AI agents for a variety of use cases.


## Features

- Powerful, customizable voice-based AI assistant
- Adaptable AI behavior to cater to specific tasks and preferences
- Pre-set questions and answers using the [Glowbom builder](https://www.glowbom.com)
- Voice input and output for a smooth and intuitive user experience
- Customizable prompts allowing you to tailor the assistant to your needs
- Capable of breaking down complex tasks into manageable steps
- Offers multiple options for each step, allowing users to choose their preferred approach
- Support for multiple languages: American English, American Spanish, Argentinian Spanish, Australian English, Brazilian Portuguese, British English, Bulgarian, Canadian French, Chinese (Simplified), Chinese (Traditional), Czech, Danish, Dutch, English, French, German, Italian, Japanese, Korean, Mexican Spanish, Norwegian, Polish, Portuguese, Russian, Spanish, Swedish, and Ukrainian. Want to add more languages? Feel free to let us know on [Twitter](https://twitter.com/glowbomcorp)

### Multilingual Support in Action

![Glowby Basic Demo](https://user-images.githubusercontent.com/2455891/232397460-6348da46-1282-438a-b9ca-3546fb6de124.gif)

## Upcoming Features

We're constantly working to improve our project and have several exciting features in development. Here's a sneak peek at what's coming soon:

### Functionality
- Autonomous mode
- Image Generation
- Local Storage

### Monetization
- Adding a paywall

Stay tuned for more updates and enhancements as we continue to grow and develop the project!


## Getting Started

### Prerequisites

- Flutter SDK (version 3.7.10 or higher)
- Dart (version 2.19.5 or higher)
- A compatible browser or device for running the web app
- [OpenAI API key](https://platform.openai.com/account/api-keys)

Glowby Basic supports **gpt-4** and **gpt-3.5-turbo**. If you don't have access to GPT-4, you can join the waitlist [here](https://help.openai.com/en/articles/7102672-how-can-i-access-gpt-4).

### Installation

1. Clone the repository:

```
git clone https://github.com/glowbom/glowby.git 
```

2. Navigate to the project directory:

```
cd app
```

3. Install dependencies:


```
flutter pub get
```

4. Run the project in your preferred environment:


```
flutter run
```

## Deployment

Glowby Basic comes with a pre-built `dist` folder, which you can deploy directly to your preferred hosting platform. Alternatively, you can build the project yourself and deploy the output. Glowby Basic is compatible with a variety of hosting services, including Netlify, Vercel, Firebase, AWS, and more. Simply follow the deployment instructions provided by your chosen hosting service.

## Customization

To customize the AI assistant's behavior and tasks, modify the default prompt in AI Settings or in the code.

#### Complex Task Prompt
```
You are Glowby, an AI assistant designed to break down complex tasks into a manageable 5-step plan. For each step, you offer the user 3 options to choose from. Once the user selects an option, you proceed to the next step based on their choice. After the user has chosen an option for the fifth step, you provide them with a customized, actionable plan based on their previous responses. You only reveal the current step and options to ensure an engaging, interactive experience.
```

#### Brainstorming Prompt
```
Generate ideas with Glowby! As a super helpful, nice, and humorous AI assistant, Glowby is ready to provide you with a concise plan and assist in executing it. With Glowby by your side, you'll never feel stuck again. Let's get brainstorming!
```

#### Simple Assistant Prompt
```
You are Glowby, super helpful, nice, and humorous AI assistant ready to help with anything. I like to joke around.
```

### Questions Pre-set

One of the powerful features of Glowby Basic is the ability to pre-set questions and answers for your AI assistant. Using [Glowbom.com](https://www.glowbom.com), you can create a knowledge base of questions and answers that your AI assistant can use to provide instant responses. If the answer to a question is not found locally, the app will make a server request to retrieve the relevant information, ensuring that users receive accurate and helpful responses.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=glowbom/glowby&type=Date)](https://star-history.com/#glowbom/glowby&Date)


## Contributing

We're excited to have you join our community and contribute to Glowby Basic! Whether you're interested in fixing bugs, adding new features, or improving documentation, your contributions are welcome. Feel free to open issues and submit pull requests on GitHub.


## License

Glowby Basic is released under the [MIT License](https://opensource.org/licenses/MIT).

## Contact

If you have any questions or need assistance, feel free to reach out to us on [Twitter](https://twitter.com/glowbomcorp).

