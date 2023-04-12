import { Timestamp } from "./Timestamp";

interface MessageProps {
  text: string;
  createdAt: Timestamp;
  userId: string;
  username?: string;
  link?: string;
}

export class Message {
  text: string;
  createdAt: Timestamp;
  userId: string;
  username?: string;
  link?: string;

  constructor({ text, createdAt, userId, username, link }: MessageProps) {
    this.text = text;
    this.createdAt = createdAt;
    this.userId = userId;
    this.username = username;
    this.link = link;
  }

  toString(): string {
    return `Message(text: ${this.text}, createdAt: ${this.createdAt}, userId: ${this.userId}, username: ${this.username}, link: ${this.link})`;
  }
}

export default Message;
