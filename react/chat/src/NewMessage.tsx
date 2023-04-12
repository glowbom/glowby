import React, { useState, useRef, useEffect } from "react";
import Ai from "./Ai";
import Message from "./Message";
import { Timestamp } from "./Timestamp";
import TextField from "@mui/material/TextField";
import IconButton from "@mui/material/IconButton";
import SendIcon from "@mui/icons-material/Send";

interface NewMessageProps {
  refresh: () => void;
  messages: Message[];
  questions?: Array<{ [key: string]: any }>;
  name?: string;
}

const NewMessage: React.FC<NewMessageProps> = ({
  refresh,
  messages,
  questions,
  name,
}) => {
  const ai = new Ai(name, questions);
  const [enteredMessage, setEnteredMessage] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const sendMessage = async () => {
    // Add the message to the messages list
    messages.unshift(
      new Message({
        text: enteredMessage.trim(),
        createdAt: Timestamp.now(),
        userId: "Me",
        username: "Me",
      })
    );

    const response = await ai.message(enteredMessage.trim());

    if (response.length > 0) {
      for (const m of response) {
        messages.unshift(m);
      }
    }

    refresh();
    setEnteredMessage("");
  };

  const handleKeyPress = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" && enteredMessage.trim()) {
      sendMessage();
    }
  };

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  return (
    <div style={{ marginTop: 8, padding: 8 }}>
      <div style={{ display: "flex" }}>
        <TextField
          inputRef={inputRef}
          fullWidth
          variant="outlined"
          size="small"
          placeholder="Send message..."
          value={enteredMessage}
          onChange={(event) => setEnteredMessage(event.target.value)}
          onKeyPress={handleKeyPress}
        />
        <IconButton
          style={{ marginLeft: 8 }}
          disabled={!enteredMessage.trim()}
          onClick={sendMessage}
        >
          <SendIcon />
        </IconButton>
      </div>
    </div>
  );
};

export default NewMessage;
