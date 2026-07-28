export type Color = 0 | 1 | 2;
export type RoomStatus = "waiting" | "playing" | "finished";

export interface Coordinate {
  row: number;
  column: number;
}

export interface Move extends Coordinate {
  player: 1 | 2;
}

export interface PlayerState {
  name: string;
  color: 1 | 2;
  connected: boolean;
  rematch: boolean;
}

export interface RoomState {
  type: "state";
  roomCode: string;
  status: RoomStatus;
  board: number[][];
  turn: 1 | 2;
  winner: Color;
  moves: Move[];
  players: PlayerState[];
  winningLine?: Coordinate[];
  undoRequester?: Color;
}

export interface RoomCredentials {
  roomCode: string;
  playerToken: string;
  color: 1 | 2;
}

export interface ServerError {
  type: "error";
  code: string;
  message: string;
}

export interface ChatMessage {
  type: "chat";
  from: 1 | 2;
  name: string;
  text: string;
}

export interface HallRoom {
  roomCode: string;
  hostName: string;
  status: RoomStatus;
  playerCount: number;
  connectedCount: number;
  moveCount: number;
  joinable: boolean;
  createdAt: number;
}
