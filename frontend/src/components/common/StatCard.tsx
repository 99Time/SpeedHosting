type StatCardProps = {
  label: string
  value: string | number
  hint: string
}

export function StatCard({ label, value, hint }: StatCardProps) {
  return (
    <section className="stat-card">
      <div className="stat-card__header">
        <p className="stat-card__label">{label}</p>
      </div>
      <div className="stat-card__body">
        <h3 className="stat-card__value">{value}</h3>
        <p className="stat-card__hint">{hint}</p>
      </div>
    </section>
  )
}
