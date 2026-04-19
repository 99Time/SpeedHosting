import { useEffect, useState } from 'react'

import { getUpdates } from '../lib/api'
import type { UpdatesResponse } from '../types/api'

function formatUpdateDate(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(parsed)
}

export function UpdatesPage() {
  const [data, setData] = useState<UpdatesResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    getUpdates()
      .then((response) => {
        if (active) {
          setData(response)
        }
      })
      .catch((requestError: Error) => {
        if (active) {
          setError(requestError.message)
        }
      })

    return () => {
      active = false
    }
  }, [])

  if (error) {
    return <section className="panel-card">Failed to load updates: {error}</section>
  }

  if (!data) {
    return (
      <div className="page-grid">
        <div className="changelog-list dash-skel" style={{ minHeight: 320 }} />
      </div>
    )
  }

  const updates = data.updates

  return (
    <div className="page-grid">
      <div className="changelog-header">
        <h2 className="changelog-header__title">Changelog</h2>
      </div>

      {updates.length === 0 ? (
        <section className="panel-card">
          <div className="empty-state">
            <h4>No updates yet</h4>
          </div>
        </section>
      ) : (
        <section className="changelog-list">
          {updates.map((update, index) => (
            <article
              className={index === 0 ? 'changelog-item changelog-item--latest' : 'changelog-item'}
              key={update.id ?? `${update.created_at}-${update.title}`}
            >
              <div className="changelog-item__meta">
                <span className={`changelog-tag changelog-tag--${update.tag.toLowerCase()}`}>{update.tag}</span>
                {index === 0 ? <span className="changelog-item__new">NEW</span> : null}
                <time className="changelog-item__date">{formatUpdateDate(update.created_at)}</time>
              </div>
              <h3 className="changelog-item__title">{update.title}</h3>
              {update.short_description ? (
                <p className="changelog-item__desc">{update.short_description}</p>
              ) : null}
            </article>
          ))}
        </section>
      )}
    </div>
  )
}