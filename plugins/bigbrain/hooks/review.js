export function draftDetail(ui, panel, refresh, apply) {
  const { Text, Input, Button } = ui
  const d = panel.draft
  const content = d.content
  const edit = panel.page === 'edit'
  const lines = [Text({ children: d.origin.scope + ' · ' + d.status + ' · revision ' + d.revision })]
  for (const [key, label] of [['memory', 'Memory'], ['evidence', 'Evidence']]) {
    const record = content[key]
    if (!record) continue
    lines.push(Text({ children: label + ' · ' + record.type, bold: true }))
    for (const field of (edit ? ['text', 'source', 'type'] : ['text', 'source'])) {
      if (edit) {
        const setValue = (value) => {
          panel.edits[key][field] = value
        }
        lines.push(
          Input({
            key: key + '-' + field,
            label: label + ' ' + field,
            value: panel.edits[key][field],
            onInput: setValue,
            onSubmit: setValue,
          }),
        )
      } else {
        lines.push(Text({ children: record[field] }))
      }
    }
  }
  for (const question of content.open_questions || []) {
    lines.push(Text({ children: 'Question · ' + question }))
  }
  for (const step of content.next_steps || []) lines.push(Text({ children: 'Next step · ' + step }))
  if (edit) {
    lines.push(Button({ key: 'apply', label: 'Apply edits', onPress: () => apply('edit', panel.edits) }))
    lines.push(Button({
      key: 'cancel',
      label: 'Cancel edits',
      onPress: () => {
        panel.page = 'review'
        refresh()
      },
    }))
  } else {
    if (d.status === 'draft') {
      lines.push(Button({
        key: 'edit',
        label: 'Edit draft',
        onPress: () => {
          panel.edits = JSON.parse(JSON.stringify(content))
          panel.page = 'edit'
          refresh()
        },
      }))
      lines.push(Button({ key: 'dismiss', label: 'Dismiss draft', onPress: () => apply('dismiss') }))
    }
    if (d.status !== 'saved' && d.status !== 'dismissed') {
      lines.push(Button({ key: 'save', label: 'Save reviewed handoff', onPress: () => apply('save') }))
    }
  }
  return lines
}
