import React, { useState } from "react";
import Message from "./Message";
import NewMessage from "./NewMessage";
import Messages from "./Messages";
import TextToSpeech from "./TextToSpeech";
import { AppBar, Toolbar, Typography } from "@mui/material";

interface ChatScreenProps {
  questions: Array<{ [key: string]: any }>;
  name: string;
  voice: boolean;
  mainColor: string;
  title: string | null;
}

const getColorFromString = (colorString: string): string => {
  switch (colorString.toLowerCase()) {
    case "black":
      return "black";
    case "blue":
      return "blue";
    case "green":
      return "green";
    case "grey":
      return "gray";
    case "red":
      return "red";
    default:
      return "blue";
  }
};

const ChatScreen: React.FC<ChatScreenProps> = ({ questions, name, voice, mainColor, title }) => {
  const textToSpeech = new TextToSpeech(); // Initialize TextToSpeech
  const [messages, setMessages] = useState<Message[]>([]); // Explicitly set the type of messages array

  // Refresh the chat screen and handle text-to-speech functionality
  const refresh = () => {
    if (voice) {
      try {
        if (messages.length > 0 && messages[0].userId === "007") {
          textToSpeech.speakText(messages[0].text);
        }
      } catch (e) {
        console.error("Error:", e); // Log the exception
      }
    }
    setMessages([...messages]);
  };

  return (
    <div style={{ display: "flex", justifyContent: "center", minHeight: "100vh", minWidth: 300 }}>
      <div style={{ display: "flex", flexDirection: "column", flexGrow: 1, maxWidth: 640 }}>
        {title && (
          <AppBar position="static" style={{ backgroundColor: getColorFromString(mainColor) }}>
            <Toolbar>
              <Typography variant="h6" component="div">
                {title}
              </Typography>
            </Toolbar>
          </AppBar>
        )}
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            justifyContent: "space-between",
            flexGrow: 1,
          }}
        >
          <Messages messages={messages} mainColor={mainColor} />
          <NewMessage refresh={refresh} messages={messages} questions={questions} name={name} />
        </div>
      </div>
    </div>
  );
};

export default ChatScreen;
