import { useState, type Key } from "react";
import { Alert, Button, Modal, Typography } from "antd";
import { useWorkspace } from "./workspaceContext";
import { useRecipientText } from "./recipientText";
import { RecipientTable } from "./RecipientTable";
import { contactJSON, contactPath } from "./recipientAPI";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
export function RecipientPicker({
  disabled,
  onApply,
}: {
  disabled?: boolean;
  onApply: (csv: string) => void;
}) {
  const context = useWorkspace(),
    t = useRecipientText(),
    job = useJob<{ csv: string }>();
  const [open, setOpen] = useState(false),
    [keys, setKeys] = useState<Key[]>([]);
  if (!context.available || !context.team) return null;
  return (
    <>
      <Button
        disabled={disabled}
        onClick={() => {
          setKeys([]);
          job.reset();
          setOpen(true);
        }}
      >
        {t.pick}
      </Button>
      <Modal
        title={t.pick}
        open={open}
        width={800}
        destroyOnHidden
        onCancel={() => {
          if (!job.busy) setOpen(false);
        }}
        footer={
          <Button
            type="primary"
            aria-label={t.apply}
            loading={job.busy}
            disabled={!keys.length}
            onClick={() =>
              void job.run(async (signal) => {
                const result = await contactJSON<{ csv: string }>(
                  contactPath(context.team!.id) + "/select",
                  { ids: keys },
                  signal,
                );
                if (!signal.aborted) {
                  onApply(result.csv);
                  setOpen(false);
                }
                return result;
              })
            }
          >
            {t.apply}
          </Button>
        }
      >
        <Alert message={t.pickHint} />
        <Typography.Paragraph>
          {t.selected}: {keys.length} / 20
        </Typography.Paragraph>
        {open && (
          <RecipientTable
            team={context.team.id}
            selection={keys}
            onSelection={setKeys}
          />
        )}
        <ErrorNotice error={job.error} />
      </Modal>
    </>
  );
}
