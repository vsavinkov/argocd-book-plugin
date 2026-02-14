import * as React from 'react';
import { BookingStatus, getStatus, bookApp, unbookApp } from './api';

const { useState, useEffect } = React;

interface BookButtonProps {
  application: {
    metadata: {
      name: string;
      namespace: string;
    };
    spec: {
      project: string;
    };
  };
}

export const BookButton: React.FC<BookButtonProps> = ({ application }) => {
  const [status, setStatus] = useState<BookingStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const appIdentifier = `${application.metadata.namespace}:${application.metadata.name}`;
  const project = application.spec.project;

  const fetchStatus = async () => {
    try {
      const s = await getStatus(appIdentifier, project);
      setStatus(s);
      setError(null);
    } catch (e: any) {
      setError(e.message);
    }
  };

  useEffect(() => {
    fetchStatus();
  }, [application.metadata.name, application.metadata.namespace]);

  const handleBook = async () => {
    setLoading(true);
    try {
      await bookApp(appIdentifier, project);
      await fetchStatus();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  const handleUnbook = async () => {
    setLoading(true);
    try {
      await unbookApp(appIdentifier, project);
      await fetchStatus();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  };

  if (!status) {
    return <span style={{ padding: '0 8px', color: '#8fa4b5' }}>Loading...</span>;
  }

  if (error) {
    return (
      <span style={{ padding: '0 8px', color: '#e96d76' }} title={error}>
        Booking Error
      </span>
    );
  }

  if (!status.booked) {
    return (
      <button
        className="argo-button argo-button--base"
        onClick={handleBook}
        disabled={loading}
        title="Book this application for exclusive use"
        style={{ display: 'flex', alignItems: 'center', gap: '4px' }}
      >
        <i className="fa fa-unlock" /> {loading ? 'Booking...' : 'Book'}
      </button>
    );
  }

  // Booked — check if it's the current user by attempting unbook (UI doesn't know current user)
  // The backend will return 403 if not the booker, so we show unbook for everyone
  // and handle the error gracefully
  return (
    <button
      className="argo-button argo-button--base"
      onClick={handleUnbook}
      disabled={loading}
      title={`Booked by ${status.bookedBy} — click to unbook (only the booker or admin can unbook)`}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '4px',
        backgroundColor: '#e96d76',
        borderColor: '#e96d76',
      }}
    >
      <i className="fa fa-lock" /> {loading ? 'Unbooking...' : `Booked: ${status.bookedBy}`}
    </button>
  );
};

export const BookButtonShouldDisplay = (): boolean => {
  return true;
};
