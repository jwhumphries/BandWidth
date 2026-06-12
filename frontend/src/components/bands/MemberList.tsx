import {useMe} from '../../hooks/auth';
import {useRemoveMember, useSetMemberRole} from '../../hooks/bands';
import type {BandDetail, BandRole} from '../../lib/types';

const roles: BandRole[] = ['viewer', 'editor', 'admin'];

export default function MemberList({band}: {band: BandDetail}) {
  const {data: me} = useMe();
  const setRole = useSetMemberRole(band.id);
  const removeMember = useRemoveMember(band.id);
  const isAdmin = band.myRole === 'admin';

  return (
    <section className="card bg-base-100 shadow">
      <div className="card-body">
        <h2 className="card-title">Members</h2>
        <ul className="flex flex-col gap-2">
          {band.members.map(member => {
            const isCreator = member.userId === band.creatorId;
            const isSelf = member.userId === me?.id;
            return (
              <li key={member.userId} className="flex items-center gap-3">
                <span className="min-w-0 flex-1 truncate">
                  {member.username}
                  {isCreator && (
                    <span className="badge badge-ghost badge-sm ml-2">
                      creator
                    </span>
                  )}
                </span>
                {isAdmin && !isCreator ? (
                  <select
                    className="select select-sm"
                    aria-label={`Role for ${member.username}`}
                    value={member.role}
                    onChange={e =>
                      setRole.mutate({
                        userId: member.userId,
                        role: e.target.value as BandRole,
                      })
                    }
                  >
                    {roles.map(r => (
                      <option key={r} value={r}>
                        {r}
                      </option>
                    ))}
                  </select>
                ) : (
                  <span className="badge badge-ghost">{member.role}</span>
                )}
                {((isAdmin && !isCreator) || (isSelf && !isCreator)) && (
                  <button
                    className="btn btn-ghost btn-xs"
                    aria-label={
                      isSelf ? 'Leave band' : `Remove ${member.username}`
                    }
                    onClick={() => removeMember.mutate(member.userId)}
                  >
                    {isSelf ? 'Leave' : '✕'}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
        {(setRole.error ?? removeMember.error) && (
          <div role="alert" className="alert alert-error">
            {(setRole.error ?? removeMember.error)?.message}
          </div>
        )}
      </div>
    </section>
  );
}
