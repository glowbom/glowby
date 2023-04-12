import React from 'react';
import { Box, Typography, Button, Paper } from '@mui/material';

interface MessageBubbleProps {
  message: string;
  username: string | undefined;
  isMe: boolean;
  link?: string | null;
  mainColor: string;
}

const getColorFromString = (colorString: string): string => {
    switch (colorString.toLowerCase()) {
      case 'black':
        return 'black';
      case 'blue':
        return 'blue';
      case 'green':
        return 'green';
      case 'grey':
        return 'gray';
      case 'red':
        return 'red';
      default:
        return 'blue';
    }
  };
  

const MessageBubble: React.FC<MessageBubbleProps> = ({ message, username, isMe, link, mainColor }) => {
  const launchLink = async () => {
    if (link) {
      window.open(link, '_blank');
    } else {
      throw new Error(`Could not launch ${link}`);
    }
  };

  const renderMessageOrLink = () => {
    if (!link) {
      return (
        <Typography style={{ color: isMe ? 'black' : 'white', textAlign: isMe ? 'right' : 'left' }}>
          {message}
        </Typography>
      );
    } else if (message === 'image') {
      return <img src={link} alt="Shared image" />;
    } else {
      return (
        <Button
          style={{
            backgroundColor: 'blue',
            color: isMe ? 'black' : 'white',
            textAlign: isMe ? 'right' : 'left',
          }}
          onClick={launchLink}
        >
          {message}
        </Button>
      );
    }
  };

  return (
    <Box display="flex" justifyContent={isMe ? 'flex-end' : 'flex-start'}>
      <Paper
        style={{
          maxWidth: 280,
          minWidth: 240,
          padding: '10px 16px',
          margin: '4px 8px',
          borderRadius: isMe
            ? '12px 12px 0 12px'
            : '12px 12px 12px 0',
          backgroundColor: isMe ? '#e0e0e0' : getColorFromString(mainColor) ,
        }}
      >
        <Typography style={{ fontWeight: 'bold', color: isMe ? 'black' : 'rgba(255, 255, 255, 0.7)', textAlign: isMe ? 'right' : 'left' }}>
          {username}
        </Typography>
        {renderMessageOrLink()}
      </Paper>
    </Box>
  );
};

export default MessageBubble;
