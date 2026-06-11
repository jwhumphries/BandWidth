import {useEffect, useState} from 'react';
import type {FormEvent} from 'react';
import {useMe, useUpdateMe} from '../../hooks/auth';

export default function AccountSettings() {
  const {data: user} = useMe();
  const updateMe = useUpdateMe();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (user) {
      setUsername(user.username);
      setEmail(user.email);
    }
  }, [user]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    setSaved(false);
    updateMe.mutate({username, email}, {onSuccess: () => setSaved(true)});
  };

  return (
    <section className="card bg-base-100 shadow">
      <form className="card-body" onSubmit={submit}>
        <h2 className="card-title">Account</h2>
        <label className="label" htmlFor="username">
          Username
        </label>
        <input
          id="username"
          className="input w-full"
          value={username}
          onChange={e => setUsername(e.target.value)}
          required
        />
        <label className="label" htmlFor="email">
          Email
        </label>
        <input
          id="email"
          type="email"
          className="input w-full"
          value={email}
          onChange={e => setEmail(e.target.value)}
          required
        />
        {updateMe.error && (
          <div role="alert" className="alert alert-error">
            {updateMe.error.message}
          </div>
        )}
        {saved && (
          <div role="status" className="alert alert-success">
            Saved
          </div>
        )}
        <div className="card-actions justify-end">
          <button className="btn btn-primary" disabled={updateMe.isPending}>
            Save
          </button>
        </div>
      </form>
    </section>
  );
}
