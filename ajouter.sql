CREATE TABLE cohorte (
    id SERIAL PRIMARY KEY,
    idExterne integer UNIQUE NOT NULL,
    nom VARCHAR(255) NOT NULL
);

CREATE TABLE user_cohorte (
    user_id   INT NOT NULL REFERENCES public.user(id) ON DELETE CASCADE,
    cohorte_id INT NOT NULL REFERENCES cohorte(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, cohorte_id)
);

CREATE TABLE feedback_cohorte (
    feedback_id   INT NOT NULL REFERENCES public.feedback(id) ON DELETE CASCADE,
    cohorte_id INT NOT NULL REFERENCES cohorte(id) ON DELETE CASCADE,
    PRIMARY KEY (feedback_id, cohorte_id)
);

-- Fonction qui copie les cohortes de l'utilisateur dans feedback_cohorte
CREATE OR REPLACE FUNCTION copy_user_cohortes_to_feedback()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO feedback_cohorte (feedback_id, cohorte_id)
    SELECT NEW.id, uc.cohorte_id
    FROM user_cohorte uc
    WHERE uc.user_id = NEW.user_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger sur l'insertion dans feedback
CREATE TRIGGER feedback_insert_copy_cohortes
AFTER INSERT ON public.feedback
FOR EACH ROW
EXECUTE FUNCTION copy_user_cohortes_to_feedback();


ALTER TABLE public.feedback DROP CONSTRAINT feedback_student_id_fkey;
ALTER TABLE public.feedback ADD CONSTRAINT feedback_student_id_fkey FOREIGN KEY (user_id) REFERENCES public."user"(id) ON DELETE CASCADE;
