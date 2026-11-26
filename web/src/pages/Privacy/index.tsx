import type { ReactNode } from "react";
import { Breadcrumbs } from "@/components/Breadcrumbs";

const ISSUES = "https://github.com/FinA1ee/dipwatch/issues";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-xl font-semibold">{title}</h2>
      <div className="space-y-2 text-[15px] leading-relaxed">{children}</div>
    </section>
  );
}

/** What Dipwatch stores about people, why, who else sees it, and how to delete it. */
export function PrivacyPage() {
  return (
    <article className="max-w-[68ch] space-y-8">
      <header className="space-y-2">
        <Breadcrumbs items={[{ label: "Search", to: "/" }, { label: "Privacy" }]} />
        <h1 className="text-[34px] font-semibold leading-tight">Privacy</h1>
        <p className="text-muted">Last updated 26 November 2026.</p>
        <p>
          Dipwatch is a personal, non-commercial project that compares store prices and sends price alerts. It
          keeps as little about you as it needs to do that. There are no ads, no analytics trackers, and
          nothing is sold or shared for marketing.
        </p>
      </header>

      <Section title="What is stored">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong>Your account</strong>, when you sign in with GitHub: your name, email address, profile
            picture link and GitHub account number, and the token GitHub issues for the sign-in. Dipwatch asks
            GitHub only for your profile and email address; it cannot see your repositories.
          </li>
          <li>
            <strong>Your sign-in session</strong>: a cookie in your browser, and on the server the session's
            IP address and browser name. Sessions are deleted 30 days after they expire.
          </li>
          <li>
            <strong>Daily search counts</strong>, to limit how many new searches one person can run: per
            account when you are signed in, per IP address when you are not. They are deleted after 30 days.
          </li>
          <li>
            <strong>What you track</strong>: the products, listings and sizes you follow, your alert rules and
            notes, your Feishu webhook address, and a record of the alerts sent to you.
          </li>
          <li>
            <strong>Searches</strong>: the text of a search is kept with the product it names, so the next
            search for it is instant. It is not kept with who searched.
          </li>
          <li>
            <strong>Your currency choice</strong> is saved in your browser only.
          </li>
        </ul>
      </Section>

      <Section title="Who else sees it">
        <ul className="list-disc space-y-2 pl-5">
          <li>
            <strong>Aliyun</strong> hosts the site and its database in Hangzhou, China.
          </li>
          <li>
            <strong>GitHub</strong> handles signing in.
          </li>
          <li>
            <strong>Feishu</strong> receives the alert messages sent to the webhook you save. If email alerts
            are on, the email provider that sends them sees your email address.
          </li>
          <li>
            <strong>The AI model provider</strong> (DeepSeek) receives search text and the store pages and
            listing titles Dipwatch reads. It never receives your email address, notes or webhook.
          </li>
        </ul>
        <p>
          Dipwatch reads public store pages. When you open a store link, you are on that store's site and its
          own privacy policy applies.
        </p>
      </Section>

      <Section title="Deleting your data">
        <p>
          "Stop tracking" on My products deletes a tracker and its rules and note. Clearing the webhook field
          removes your webhook. To delete your account and everything tied to it, open an issue at{" "}
          <a href={ISSUES} className="font-medium underline" target="_blank" rel="noopener noreferrer">
            github.com/FinA1ee/dipwatch/issues
          </a>{" "}
          from the GitHub account you sign in with; it is deleted within 7 days. Daily backups are kept for 7
          days, so deleted data is gone from them a week later.
        </p>
      </Section>

      <Section title="Changes">
        <p>If this page changes, the date at the top changes with it.</p>
      </Section>
    </article>
  );
}
