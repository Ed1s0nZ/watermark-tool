import { Alert, Button } from "antd";
import { useApprovalText } from "./approvalText";
import { useJobText } from "./jobText";
export function ApprovalRedirect({
  onNavigate,
}: {
  onNavigate: (page: string) => void;
}) {
  const t = useApprovalText(),
    jt = useJobText();
  return (
    <Alert
      type="info"
      showIcon
      message={t.required}
      description={t.settingHint}
      action={<Button onClick={() => onNavigate("jobs")}>{jt.jobs}</Button>}
    />
  );
}
