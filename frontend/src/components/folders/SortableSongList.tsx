import {DndContext, closestCenter} from '@dnd-kit/core';
import type {DragEndEvent} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import {GripVertical} from 'lucide-react';
import SongRow from '../songs/SongRow';
import type {SongListItem} from '../../lib/types';

function SortableSongRow({
  song,
  onPracticed,
}: {
  song: SongListItem;
  onPracticed: (id: number, date: string) => void;
}) {
  const {attributes, listeners, setNodeRef, transform, transition} =
    useSortable({id: song.id});
  return (
    <div
      ref={setNodeRef}
      style={{transform: CSS.Transform.toString(transform), transition}}
      className="flex items-center gap-1"
    >
      <button
        className="text-base-content/30 hover:text-base-content/70 cursor-grab touch-none px-1"
        aria-label={`Reorder ${song.title}`}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="size-4" />
      </button>
      <div className="min-w-0 flex-1">
        <ul>
          <SongRow
            song={song}
            linkTo={`/songs/${song.id}`}
            onPracticed={onPracticed}
          />
        </ul>
      </div>
    </div>
  );
}

// SortableSongList renders folder songs in folder order with drag reorder.
export default function SortableSongList({
  songs,
  onPracticed,
  onReorder,
}: {
  songs: SongListItem[];
  onPracticed: (id: number, date: string) => void;
  onReorder: (songIds: number[]) => void;
}) {
  const dragEnd = (event: DragEndEvent) => {
    const {active, over} = event;
    if (!over || active.id === over.id) return;
    const ids = songs.map(s => s.id);
    const from = ids.indexOf(Number(active.id));
    const to = ids.indexOf(Number(over.id));
    if (from === -1 || to === -1) return;
    ids.splice(to, 0, ...ids.splice(from, 1));
    onReorder(ids);
  };

  return (
    <DndContext collisionDetection={closestCenter} onDragEnd={dragEnd}>
      <SortableContext
        items={songs.map(s => s.id)}
        strategy={verticalListSortingStrategy}
      >
        <div className="flex flex-col gap-2">
          {songs.map(song => (
            <SortableSongRow
              key={song.id}
              song={song}
              onPracticed={onPracticed}
            />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}
