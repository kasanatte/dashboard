import { fetchEventSource } from '@microsoft/fetch-event-source';

export interface StreamResponse {
  type: 'text' | 'tool_call' | 'tool_result' | 'completion';
  content: any;
}

export interface MCPStatus {
  enabled: boolean;
  initialized: boolean;
  servers: number;
  total_tools: number;
  [key: string]: any;
}

export const getAssistantStream = (
  message: string,
  onMessage: (data: StreamResponse) => void,
  onError: (error: any) => void,
  onClose: () => void,
) => {
  fetchEventSource('/api/v1/assistant', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ message }),
    onmessage(ev: { data: string }) {
      try {
        const parsedData = JSON.parse(ev.data) as StreamResponse;
        onMessage(parsedData);
      } catch (error) {
        // Fallback for plain text messages
        onMessage({ type: 'text', content: ev.data });
      }
    },
    onerror(err: any) {
      onError(err);
    },
    onclose() {
      onClose();
    },
  });
};

export const getAssistantStatus = async (): Promise<MCPStatus> => {
  const response = await fetch('/api/v1/assistant/status');
  if (!response.ok) {
    throw new Error('Failed to fetch assistant status');
  }
  return response.json();
};
