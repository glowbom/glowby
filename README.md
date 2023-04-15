# Glowby Basic - Develop and Deploy Voice-Enabled AI Agents

Glowby Basic is an open-source platform for developing and deploying powerful voice-based AI agents that can assist users in breaking down complex tasks and solving them efficiently. Built using Flutter, Glowby Basic provides a seamless web app experience with an intuitive voice interface.

## Live Demo

Experience Glowby Basic in action with our live demo hosted on GitHub Pages [here](https://glowbom.github.io/glowby-basic/).

## Overview

This project offers an easy-to-use platform for creating customizable AI agents like Glowby, a witty AI assistant that assists users in building apps on [Glowbom.com](https://www.glowbom.com). By open-sourcing the Flutter-based chat component, we aim to foster a community-driven ecosystem to build diverse AI agents for a variety of use cases.

## Features

- User-friendly platform to create voice-based AI agents
- Customizable AI behavior to cater to specific tasks and preferences
- Breaks down complex tasks into 5 manageable steps
- Offers 3 options for each step, allowing users to choose their preferred approach
- Interactive and engaging experience with personalized, actionable plans based on user choices

## Getting Started

### Prerequisites

- Flutter SDK (version 3.7.10 or higher)
- Dart (version 2.19.5 or higher)
- A compatible browser or device for running the web app

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

To customize the AI agent's behavior and tasks, modify the default prompt in AI Settings or in the code.

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

