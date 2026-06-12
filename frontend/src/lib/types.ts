export interface User {
  id: number;
  username: string;
  email: string;
  totpEnabled: boolean;
}

export interface AuthFeatures {
  passwordReset: boolean;
}

export interface TwoFactorSetupResponse {
  secret: string;
  otpauthUrl: string;
}

export interface TwoFactorVerifyResponse {
  backupCodes: string[];
}

export type SongStatus = 'not_learned' | 'learning' | 'learned' | 'nailed';

export interface SongListItem {
  id: number;
  title: string;
  artist: string;
  status: SongStatus;
  lastPracticedAt: string;
  practiceCount: number;
}

export interface Resource {
  id: number;
  url: string;
  label: string;
}

export interface SongDetail extends SongListItem {
  notes: string;
  resources: Resource[];
}

export interface PracticeStats {
  lastPracticedAt: string;
  practiceCount: number;
}

export interface Folder {
  id: number;
  name: string;
  position: number;
  songIds: number[];
}

export type BandRole = 'viewer' | 'editor' | 'admin';

export interface BandSummary {
  id: number;
  name: string;
  role: BandRole;
  memberCount: number;
}

export interface BandMemberInfo {
  userId: number;
  username: string;
  role: BandRole;
}

export interface BandDetail {
  id: number;
  name: string;
  creatorId: number;
  myRole: BandRole;
  members: BandMemberInfo[];
}

export interface MyInvite {
  id: number;
  bandId: number;
  bandName: string;
  role: BandRole;
}

export interface BandInviteInfo {
  id: number;
  role: BandRole;
  invitedUsername: string | null;
  isLink: boolean;
  expiresAt: string;
}

export interface CreatedInvite {
  id?: number;
  role: BandRole;
  invitedUsername?: string;
  token?: string;
  isLink?: boolean;
}
