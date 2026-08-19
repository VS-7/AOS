type SettingsFieldLabelProps = {
  label: string;
  description: string;
};

/**
 * Standard label + description stack used across settings FormSectionItems.
 */
export function SettingsFieldLabel({ label, description }: SettingsFieldLabelProps) {
  return (
    <div className="space-y-0.5">
      <p className="text-sm font-medium text-foreground">{label}</p>
      <p className="text-sm text-muted-foreground">{description}</p>
    </div>
  );
}
