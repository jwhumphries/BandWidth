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
