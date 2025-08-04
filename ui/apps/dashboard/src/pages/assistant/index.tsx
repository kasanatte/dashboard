import React, { useState, useRef, useEffect } from 'react';
import {
  Input,
  Button,
  List,
  Card,
  Spin,
  Alert,
  Collapse,
  Badge,
  Typography,
  Space,
  Tag,
} from 'antd';
import { Icons } from '@/components/icons';
import {
  getAssistantStream,
  getAssistantStatus,
  StreamResponse,
  MCPStatus,
} from '@/services/assistant';

interface ToolCall {
  server: string;
  tool: string;
  arguments: Record<string, any>;
}

interface ToolResult {
  tool: string;
  result: any;
  isError: boolean;
}

interface Message {
  id: string;
  text: string;
  sender: 'user' | 'bot';
  toolCall?: ToolCall;
  toolResult?: ToolResult;
}

const { Text } = Typography;

const AssistantPage: React.FC = () => {
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [mcpStatus, setMcpStatus] = useState<MCPStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(true);
  const currentBotMessageRef = useRef<Message | null>(null);
  useEffect(() => {
    const fetchStatus = async () => {
      try {
        setStatusLoading(true);
        const status = await getAssistantStatus();
        setMcpStatus(status);
      } catch (error) {
        console.error('Failed to fetch MCP status:', error);
        setMcpStatus({
          enabled: false,
          initialized: false,
          servers: 0,
          total_tools: 0,
        });
      } finally {
        setStatusLoading(false);
      }
    };

    fetchStatus();
    // Refresh status every 30 seconds
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleSendMessage = () => {
    if (inputValue.trim() === '') return;

    const userMessage: Message = {
      id: Date.now().toString(),
      text: inputValue,
      sender: 'user',
    };
    setMessages((prevMessages) => [...prevMessages, userMessage]);
    setInputValue('');
    setIsLoading(true);

    const botMessage: Message = {
      id: (Date.now() + 1).toString(),
      text: '',
      sender: 'bot',
    };
    setMessages((prevMessages) => [...prevMessages, botMessage]);
    currentBotMessageRef.current = botMessage;

    let currentText = '';

    getAssistantStream(
      inputValue,
      (data: StreamResponse) => {
        switch (data.type) {
          case 'text':
            currentText += data.content;
            setMessages((prevMessages) => {
              const newMessages = [...prevMessages];
              const lastMessage = newMessages[newMessages.length - 1];
              if (lastMessage && lastMessage.sender === 'bot') {
                lastMessage.text = currentText;
              }
              return newMessages;
            });
            break;

          case 'tool_call':
            const toolCall = data.content as ToolCall;
            setMessages((prevMessages) => {
              const newMessages = [...prevMessages];
              const lastMessage = newMessages[newMessages.length - 1];
              if (lastMessage && lastMessage.sender === 'bot') {
                lastMessage.toolCall = toolCall;
                lastMessage.text =
                  currentText + `🔧 Using tool: ${toolCall.tool}`;
              }
              return newMessages;
            });
            break;

          case 'tool_result':
            const toolResult = data.content as ToolResult;
            setMessages((prevMessages) => {
              const newMessages = [...prevMessages];
              const lastMessage = newMessages[newMessages.length - 1];
              if (lastMessage && lastMessage.sender === 'bot') {
                lastMessage.toolResult = toolResult;
                lastMessage.text = currentText;
              }
              return newMessages;
            });
            break;

          case 'completion':
            setIsLoading(false);
            currentBotMessageRef.current = null;
            break;
        }
      },
      (error) => {
        console.error('Stream error:', error);
        setIsLoading(false);
      },
      () => {
        setIsLoading(false);
        currentBotMessageRef.current = null;
      },
    );
  };

  const renderStatusIndicator = () => {
    if (statusLoading) {
      return <Spin size="small" />;
    }

    if (!mcpStatus) {
      return <Tag color="red">MCP Unavailable</Tag>;
    }

    if (mcpStatus.enabled && mcpStatus.initialized) {
      return (
        <Space>
          <Badge status="success" text="MCP Ready" />
          <Tag color="green">{mcpStatus.total_tools} tools available</Tag>
        </Space>
      );
    }

    return (
      <Space>
        <Badge status="error" text="MCP Not Ready" />
        <Tag color="orange">Limited functionality</Tag>
      </Space>
    );
  };

  const renderMessageContent = (message: Message) => {
    return (
      <div>
        <div>{message.text}</div>
        {message.toolCall && (
          <Collapse size="small" style={{ marginTop: 8 }}>
            <Collapse.Panel header="🔧 Tool Call" key="1">
              <Card
                size="small"
                style={{ backgroundColor: '#f0f8ff', border: 'none' }}
              >
                <div>
                  <strong>Tool:</strong> {message.toolCall.tool}
                </div>
                <div>
                  <strong>Arguments:</strong>
                </div>
                <pre
                  style={{
                    backgroundColor: '#f5f5f5',
                    padding: 8,
                    borderRadius: 4,
                    margin: 0,
                  }}
                >
                  {JSON.stringify(message.toolCall.arguments, null, 2)}
                </pre>
              </Card>
            </Collapse.Panel>
          </Collapse>
        )}
        {message.toolResult && (
          <Collapse size="small" style={{ marginTop: 8 }}>
            <Collapse.Panel header="📊 Tool Result" key="1">
              <Card
                size="small"
                style={{
                  border: 'none',
                  backgroundColor: message.toolResult.isError
                    ? '#fff2f0'
                    : '#f6ffed',
                }}
              >
                {message.toolResult.isError ? (
                  <Alert
                    message="Error"
                    description={message.toolResult.result}
                    type="error"
                    showIcon
                  />
                ) : (
                  <div>
                    {(() => {
                      let displayResult = message.toolResult.result;

                      // try to parse nested JSON string
                      if (typeof displayResult === 'string') {
                        try {
                          const parsed = JSON.parse(displayResult);
                          // if it's an array and contains a text field, try to parse further
                          if (
                            Array.isArray(parsed) &&
                            parsed.length > 0 &&
                            parsed[0].text
                          ) {
                            try {
                              const innerParsed = JSON.parse(parsed[0].text);
                              displayResult = innerParsed;
                            } catch {
                              displayResult = parsed;
                            }
                          } else {
                            displayResult = parsed;
                          }
                        } catch {
                          // if parse failed, keep the original string
                        }
                      }

                      // special handling for cluster list
                      if (
                        displayResult &&
                        typeof displayResult === 'object' &&
                        displayResult.clusters
                      ) {
                        return (
                          <div>
                            <div
                              style={{ marginBottom: 8, fontWeight: 'bold' }}
                            >
                              Found Clusters:
                            </div>
                            <ul style={{ margin: 0, paddingLeft: 20 }}>
                              {displayResult.clusters.map(
                                (cluster: string, index: number) => (
                                  <li key={index}>{cluster}</li>
                                ),
                              )}
                            </ul>
                          </div>
                        );
                      }

                      return (
                        <pre
                          style={{
                            backgroundColor: 'transparent',
                            padding: 8,
                            borderRadius: 4,
                            margin: 0,
                            maxHeight: 300,
                            overflow: 'auto',
                          }}
                        >
                          {typeof displayResult === 'string'
                            ? displayResult
                            : JSON.stringify(displayResult, null, 2)}
                        </pre>
                      );
                    })()}
                  </div>
                )}
              </Card>
            </Collapse.Panel>
          </Collapse>
        )}
      </div>
    );
  };

  return (
    <div
      style={{
        padding: '20px',
        maxWidth: '800px',
        margin: 'auto',
        height: 'calc(100vh - 200px)',
      }}
    >
      <div
        style={{
          marginBottom: 16,
          padding: 16,
          backgroundColor: '#f9f9f9',
          borderRadius: 8,
          border: '1px solid #e8e8e8',
        }}
      >
        <Space>
          <Text strong>Karmada AI Assistant</Text>
          {renderStatusIndicator()}
        </Space>
        {!mcpStatus?.enabled && !statusLoading && (
          <div style={{ marginTop: 8 }}>
            <Alert
              message="MCP Server Not Available"
              description="The Karmada MCP server is not available. The assistant will provide general guidance but cannot access your clusters directly."
              type="warning"
              showIcon
              style={{ marginTop: 8 }}
            />
          </div>
        )}
      </div>

      <div
        style={{
          height: 'calc(100% - 120px)',
          overflowY: 'auto',
          marginBottom: 20,
        }}
      >
        {messages.length === 0 && (
          <div style={{ textAlign: 'center', padding: 40, color: '#666' }}>
            <Icons.bot style={{ fontSize: 48, marginBottom: 16 }} />
            <h3>Welcome to Karmada AI Assistant</h3>
            <p>
              Ask me anything about your Karmada clusters and I'll help you
              manage them!
            </p>
            <div style={{ marginTop: 20, fontSize: 12, color: '#999' }}>
              <p>Try asking:</p>
              <ul style={{ textAlign: 'left', display: 'inline-block' }}>
                <li>"List all my clusters"</li>
                <li>"Show me deployments in namespace default"</li>
                <li>"What's the status of my pods?"</li>
                <li>"Create a new deployment"</li>
              </ul>
            </div>
          </div>
        )}
        <List
          dataSource={messages}
          renderItem={(item) => (
            <List.Item
              style={{ textAlign: item.sender === 'user' ? 'right' : 'left' }}
            >
              <List.Item.Meta
                avatar={item.sender === 'bot' ? <Icons.bot /> : <Icons.user />}
                title={item.sender === 'user' ? 'You' : 'Assistant'}
                description={renderMessageContent(item)}
              />
            </List.Item>
          )}
        />
        {isLoading && (
          <div style={{ textAlign: 'center', padding: 20 }}>
            <Spin />
          </div>
        )}
      </div>
      <div style={{ display: 'flex' }}>
        <Input
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onPressEnter={handleSendMessage}
          placeholder={
            mcpStatus?.enabled
              ? 'Ask me about your Karmada clusters...'
              : 'Ask me about Karmada...'
          }
          disabled={isLoading}
        />
        <Button
          onClick={handleSendMessage}
          type="primary"
          style={{ marginLeft: '10px' }}
          disabled={isLoading}
          icon={<Icons.send />}
        >
          Send
        </Button>
      </div>
    </div>
  );
};

export default AssistantPage;
