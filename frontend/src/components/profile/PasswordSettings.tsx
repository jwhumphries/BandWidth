import {useState} from 'react';
import type {FormEvent} from 'react';
import {useChangePassword} from '../../hooks/auth';

export default function PasswordSettings() {
  const changePassword = useChangePassword();
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [saved, setSaved] = useState(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setSaved(false);
    changePassword.mutate(
      {currentPassword, newPassword},
      {
        onSuccess: () => {
          setSaved(true);
          setCurrentPassword('');
          setNewPassword('');
        },
      },
    );
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Password</h2>
        <label className="label" htmlFor="current-password">
          Current password
        </label>
        <input
          id="current-password"
          type="password"
          className="input w-full"
          value={currentPassword}
          onChange={e => {
            setSaved(false);
            setCurrentPassword(e.target.value);
          }}
          disabled={changePassword.isPending}
          autoComplete="current-password"
          required
        />
        <label className="label" htmlFor="new-password">
          New password
        </label>
        <input
          id="new-password"
          type="password"
          className="input w-full"
          value={newPassword}
          onChange={e => {
            setSaved(false);
            setNewPassword(e.target.value);
          }}
          disabled={changePassword.isPending}
          minLength={8}
          autoComplete="new-password"
          required
        />
        {changePassword.error && (
          <div role="alert" className="alert alert-error">
            {changePassword.error.message}
          </div>
        )}
        {saved && (
          <div role="status" className="alert alert-success">
            Password changed
          </div>
        )}
        <div className="card-actions justify-end">
          <button
            className="btn btn-primary"
            disabled={changePassword.isPending}
          >
            Change password
          </button>
        </div>
      </form>
    </section>
  );
}
