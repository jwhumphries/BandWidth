import AccountSettings from '../components/profile/AccountSettings';
import PasswordSettings from '../components/profile/PasswordSettings';
import TwoFactorSettings from '../components/profile/TwoFactorSettings';

export default function ProfilePage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-3xl font-bold">Profile</h1>
      <AccountSettings />
      <PasswordSettings />
      <TwoFactorSettings />
    </div>
  );
}
