import { useEffect, useState } from "react";
import type { Key, ReactNode } from "react";
import { Input, Select, Space, Table, Tag } from "antd";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import {
  contactJSON,
  contactPath,
  type Contact,
  type ContactSearch,
} from "./recipientAPI";
import { useRecipientText } from "./recipientText";
export function RecipientTable({
  team,
  revision = 0,
  selection,
  onSelection,
  actions,
}: {
  team: string;
  revision?: number;
  selection?: Key[];
  onSelection?: (keys: Key[]) => void;
  actions?: (item: Contact) => ReactNode;
}) {
  const t = useRecipientText(),
    query = useJob<ContactSearch>();
  const [q, setQ] = useState(""),
    [group, setGroup] = useState(""),
    [page, setPage] = useState(1);
  useEffect(() => {
    query.reset();
    const timer = setTimeout(
      () =>
        void query.run((signal) =>
          contactJSON(
            contactPath(team) + "/search",
            { q, group, page, page_size: 20 },
            signal,
          ),
        ),
      200,
    );
    return () => {
      clearTimeout(timer);
      query.reset();
    };
    // The request is replaced when a query, page or explicit refresh changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [team, q, group, page, revision]);
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Input.Search
          aria-label={t.search}
          placeholder={t.search}
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setPage(1);
          }}
          maxLength={128}
        />
        <Select
          aria-label={t.groups}
          style={{ minWidth: 160 }}
          value={group}
          onChange={(v) => {
            setGroup(v);
            setPage(1);
          }}
          options={[
            { value: "", label: t.all },
            ...(query.value?.groups ?? []).map((value) => ({
              value,
              label: value,
            })),
          ]}
        />
      </Space>
      <ErrorNotice error={query.error} />
      <Table<Contact>
        rowKey="id"
        loading={query.busy}
        dataSource={query.value?.items ?? []}
        scroll={{ x: 550 }}
        rowSelection={
          onSelection
            ? {
                hideSelectAll: true,
                selectedRowKeys: selection,
                preserveSelectedRowKeys: true,
                onChange: onSelection,
                getCheckboxProps: (row) => ({
                  disabled:
                    (selection?.length ?? 0) >= 20 &&
                    !selection?.includes(row.id),
                }),
              }
            : undefined
        }
        pagination={{
          current: page,
          pageSize: 20,
          total: query.value?.total ?? 0,
          showSizeChanger: false,
          onChange: setPage,
        }}
        columns={[
          { title: t.id, dataIndex: "recipient_id" },
          { title: t.name, dataIndex: "name" },
          {
            title: t.groups,
            render: (_, row) => (
              <>
                {row.groups.map((g) => (
                  <Tag key={g}>{g}</Tag>
                ))}
              </>
            ),
          },
          ...(actions
            ? [
                {
                  title: t.actions,
                  render: (_: unknown, row: Contact) => actions(row),
                },
              ]
            : []),
        ]}
      />
    </Space>
  );
}
