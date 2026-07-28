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
