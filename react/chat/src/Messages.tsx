import MessageBubble from './MessageBubble';
import { Message } from './Message';

interface MessagesProps {
  messages: Message[];
  mainColor: string;
}

const Messages = ({ messages, mainColor }: MessagesProps) => {
  const processMessageText = (messageText: string): string => {
    const languagePrefixes = [
        'Italian: ',
        'German: ',
        'Portuguese: ',
        'Dutch: ',
        'Russian: ',
        'American Spanish: ',
        'Mexican Spanish: ',
        'Canadian French: ',
        'French: ',
        'Spanish: ',
        'American English: ',
        'Australian English: ',
        'British English: ',
        'English: ',
    ];

    for (const prefix of languagePrefixes) {
      messageText = messageText.replaceAll(prefix, '');
    }

    return messageText;
  };

  return (
    <div style={{ overflowY: 'scroll', display: 'flex', flexDirection: 'column-reverse' }}>
      {messages.map((message, index) => {
        const processedText = processMessageText(message.text);
        return (
          <MessageBubble
            key={message.createdAt.toString()}
            message={processedText}
            username={message.username}
            isMe={message.userId === 'Me'}
            link={message.link}
            mainColor={mainColor}
          />
        );
      })}
    </div>
  );
};

export default Messages;
