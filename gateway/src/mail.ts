import nodemailer from "nodemailer";

export type Mail = { to: string; subject: string; text: string };

export type Mailer = { send: (mail: Mail) => Promise<void> };

/**
 * SMTP mailer, e.g. SMTP_URL=smtps://user%40qq.com:<app code>@smtp.qq.com:465.
 * The URL holds the password, so it must only come from the environment.
 */
export function smtpMailer(smtpUrl: string, from: string): Mailer {
  const transport = nodemailer.createTransport(smtpUrl);
  return {
    send: async (mail) => {
      await transport.sendMail({ from, ...mail });
    },
  };
}

/** Development only: prints the email, so magic links work without SMTP. */
export function logMailer(): Mailer {
  return {
    send: async (mail) => {
      console.info(
        JSON.stringify({
          msg: "mail (not sent: SMTP_URL unset)",
          to: mail.to,
          subject: mail.subject,
          text: mail.text,
        }),
      );
    },
  };
}
