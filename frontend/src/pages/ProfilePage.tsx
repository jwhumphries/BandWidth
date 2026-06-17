import BandsCard from '../components/bands/BandsCard';
import AccountSettings from '../components/profile/AccountSettings';
import PasswordSettings from '../components/profile/PasswordSettings';
import TwoFactorSettings from '../components/profile/TwoFactorSettings';

export default function ProfilePage() {
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <h1 className="font-display text-3xl font-bold tracking-tight">
        Profile
      </h1>
      <AccountSettings />
      <PasswordSettings />
      <TwoFactorSettings />
      <BandsCard />
    </div>
  );
}
