import {useEffect, useRef, useState} from 'react';
import {Calendar, ChevronLeft, ChevronRight} from 'lucide-react';
import {format, parseISO} from 'date-fns';
import {DayPicker} from 'react-day-picker';

interface DatePickerProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  'aria-label': string;
}

export default function DatePicker({
  value,
  onChange,
  placeholder = 'Select date',
  'aria-label': ariaLabel,
}: DatePickerProps) {
  const detailsRef = useRef<HTMLDetailsElement>(null);
  const selected = value ? parseISO(value) : undefined;
  const [month, setMonth] = useState<Date>(selected ?? new Date());

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        detailsRef.current &&
        !detailsRef.current.contains(event.target as Node)
      ) {
        detailsRef.current.removeAttribute('open');
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSelect = (date: Date | undefined) => {
    if (date) {
      onChange(format(date, 'yyyy-MM-dd'));
      detailsRef.current?.removeAttribute('open');
    }
  };

  return (
    <details className="dropdown" ref={detailsRef}>
      <summary
        className="input input-sm flex w-44 cursor-pointer list-none items-center gap-2 marker:hidden"
        aria-label={ariaLabel}
      >
        <Calendar className="size-4 opacity-70" />
        <span className={selected ? '' : 'opacity-50'}>
          {selected ? format(selected, 'MMM d, yyyy') : placeholder}
        </span>
      </summary>

      <div className="dropdown-content bg-base-100 rounded-box z-10 mt-1 p-2 shadow-xl">
        <DayPicker
          mode="single"
          month={month}
          onMonthChange={setMonth}
          selected={selected}
          onSelect={handleSelect}
          showOutsideDays
          weekStartsOn={1}
          className="react-day-picker"
          components={{
            Chevron: ({orientation}) =>
              orientation === 'left' ? (
                <ChevronLeft className="size-4" />
              ) : (
                <ChevronRight className="size-4" />
              ),
          }}
          footer={
            <div className="border-base-300 mt-2 flex justify-center border-t pt-2">
              <button
                type="button"
                className="btn btn-ghost btn-xs"
                onClick={() => setMonth(new Date())}
              >
                Today
              </button>
            </div>
          }
        />
      </div>
    </details>
  );
}
