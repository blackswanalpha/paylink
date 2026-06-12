#!/usr/bin/env python3
"""Generate the per-beat narration clips for the demo video (Kenyan neural voice)."""
import asyncio, json, subprocess, edge_tts

VOICE = "en-KE-ChilembaNeural"

BEATS = {
    "t1_title": "Across East Africa, billions flow through mobile money every day. "
                "LinkMint lets anyone accept it — with nothing more than a link.",
    "t2_problem": "Today, to accept a digital payment, a business needs a licence, "
                  "locked-up capital, and one chosen rail. And fixed fees make small "
                  "payments simply not worth collecting.",
    "t3_idea": "LinkMint changes the model. A PayLink: scan it, pay with M-Pesa, and "
               "settle on-chain. No account, no app — and the money never touches us.",
    "t4_create": "Here's the real product, running now. I create a PayLink — receiver, "
                 "amount, and expiry — and LinkMint mints it on our own blockchain.",
    "t5_pay": "Now the customer pays — M-Pesa Pay Bill, with the PayLink as the account "
              "number. Nothing to download, and no account to open.",
    "t6_settle": "Watch the settlement. Our adapter turns the payment into a signed proof, "
                 "the chain verifies it, and there it is — verified. Settled on-chain in "
                 "seconds, with a real transaction hash and a tamper-evident trail.",
    "t7_traction": "And this isn't a prototype. Twelve services, a custom blockchain, an "
                   "M-Pesa adapter, an SDK, and this app — with an end-to-end settlement "
                   "test passing in continuous integration, at eighty percent coverage.",
    "t8_close": "Phase one is done. I'm Victor Kamande. LinkMint — pay anyone, through "
                "any rail, with a link.",
}


async def main():
    for key, text in BEATS.items():
        await edge_tts.Communicate(text, VOICE).save(f"vo/{key}.mp3")
    durations = {}
    for key in BEATS:
        out = subprocess.check_output([
            "ffprobe", "-v", "error", "-show_entries", "format=duration",
            "-of", "default=nokey=1:noprint_wrappers=1", f"vo/{key}.mp3",
        ]).decode().strip()
        durations[key] = round(float(out), 3)
    with open("vo/durations.json", "w") as f:
        json.dump(durations, f, indent=2)
    print(json.dumps(durations, indent=2))
    print("TOTAL", round(sum(durations.values()), 2), "s")


asyncio.run(main())
