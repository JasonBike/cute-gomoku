export interface UserProfile {
  id: string;
  nickname: string;
  avatar: string;
  rating: number;
  wins: number;
  losses: number;
  draws: number;
  createdAt: number;
}

export interface SessionState {
  user: UserProfile;
  expiresAt: number;
}
